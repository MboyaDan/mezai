package tasks_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/dbtest"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
	"github.com/MboyaDan/mezai/backend/seed"
)

// crowd creates n active, verified contributors (fake, .invalid emails).
func crowd(t *testing.T, pool *db.Pool, n int) []*auth.Principal {
	t.Helper()
	ctx := context.Background()
	out := make([]*auth.Principal, 0, n)
	err := pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		for i := range n {
			p := &auth.Principal{Kind: auth.KindContributor}
			if err := tx.QueryRow(ctx, `INSERT INTO users (email, display_name) VALUES ($1, $2) RETURNING id`,
				fmt.Sprintf("crowd%03d@sample.invalid", i), fmt.Sprintf("Sample Crowd %03d", i)).Scan(&p.UserID); err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `INSERT INTO contributor_profiles (user_id, public_code, status, identity_status)
				VALUES ($1, 'C-' || lpad(nextval('contributor_public_code_seq')::text, 4, '0'), 'active', 'verified') RETURNING id`,
				p.UserID).Scan(&p.ContributorID); err != nil {
				return err
			}
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func setup(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	if _, err := seed.Run(ctx, pool, seed.Options{Password: "sample-password-only"}); err != nil {
		t.Fatal(err)
	}
	return pool, ctx
}

var yes = json.RawMessage(`{"labels":["Yes"]}`)

// TestManyPeopleTasking runs 60 contributors at once, each taking and
// submitting tasks until none are left, and checks the invariants that must
// hold whatever the interleaving (AC-06 under load).
func TestManyPeopleTasking(t *testing.T) {
	pool, ctx := setup(t)
	const workers, k, regular, gold = 60, 3, 120, 20
	project := bigProject(t, pool, k, regular, gold, 0.10)
	people := crowd(t, pool, workers)
	svc := tasks.NewService(pool)

	var submitted, conflicts atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	start := time.Now()
	for i, p := range people {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; ; n++ {
				task, err := svc.Next(ctx, p, project)
				if errors.Is(err, tasks.ErrNoTasks) {
					return
				}
				if err != nil {
					errs <- fmt.Errorf("worker %d next: %w", i, err)
					return
				}
				if _, err := svc.Submit(ctx, p, task.ID, fmt.Sprintf("load-%d-%d", i, n),
					tasks.SubmitInput{Answer: yes}, auth.Meta{}); err != nil {
					if errors.Is(err, tasks.ErrReservationClosed) {
						conflicts.Add(1)
						continue
					}
					errs <- fmt.Errorf("worker %d submit: %w", i, err)
					return
				}
				submitted.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	took := time.Since(start)
	t.Logf("%d workers submitted %d tasks in %s (%.0f a second); %d reservations closed under them",
		workers, submitted.Load(), took.Round(time.Millisecond), float64(submitted.Load())/took.Seconds(), conflicts.Load())

	var overK, notK, dupes, multiReserved int
	_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM items i WHERE i.project_id = $1
			AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
			AND (SELECT count(*) FROM assignments a WHERE a.item_id = i.id AND a.status IN ('reserved', 'submitted')) > $2`, project, k).Scan(&overK)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM items i WHERE i.project_id = $1
			AND NOT EXISTS (SELECT 1 FROM gold_items g WHERE g.item_id = i.id)
			AND (i.submission_count <> $2 OR i.status <> 'awaiting_consensus')`, project, k).Scan(&notK)
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM assignments WHERE project_id = $1
			GROUP BY item_id, contributor_id HAVING count(*) > 1) d`, project).Scan(&dupes)
		return tx.QueryRow(ctx, `SELECT count(*) FROM (SELECT 1 FROM assignments WHERE project_id = $1 AND status = 'reserved'
			GROUP BY contributor_id HAVING count(*) > 1) d`, project).Scan(&multiReserved)
	})
	if overK != 0 || notK != 0 || dupes != 0 || multiReserved != 0 {
		t.Fatalf("items over k: %d; regular items not finished at exactly k: %d; duplicate item per person: %d; people holding two tasks: %d",
			overK, notK, dupes, multiReserved)
	}
	if want := int64(regular * k); submitted.Load() < want {
		t.Fatalf("submitted %d, want at least %d regular submissions", submitted.Load(), want)
	}
}

// TestSameContributorRacing: one person clicking "next" ten times at once
// gets one task, not ten.
func TestSameContributorRacing(t *testing.T) {
	pool, ctx := setup(t)
	project := bigProject(t, pool, 2, 30, 5, 0.1)
	p := crowd(t, pool, 1)[0]
	svc := tasks.NewService(pool)
	ids := make([]uuid.UUID, 10)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := svc.Next(ctx, p, project)
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = task.ID
		}()
	}
	wg.Wait()
	for _, id := range ids[1:] {
		if id != ids[0] {
			t.Fatalf("one person got several tasks: %v", ids)
		}
	}
}

// TestSubmitRaces: the same key sent ten times at once gives one submission
// and the same answer every time (AC-07 under a double click on a slow
// line); ten different keys at once give one submission and nine refusals.
func TestSubmitRaces(t *testing.T) {
	pool, ctx := setup(t)
	project := bigProject(t, pool, 1, 10, 0, 0)
	people := crowd(t, pool, 2)
	svc := tasks.NewService(pool)

	race := func(p *auth.Principal, key func(int) string) ([]tasks.Result, []error, uuid.UUID) {
		task, err := svc.Next(ctx, p, project)
		if err != nil {
			t.Fatal(err)
		}
		res := make([]tasks.Result, 10)
		errs := make([]error, 10)
		var wg sync.WaitGroup
		for i := range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(time.Duration(rand.IntN(3)) * time.Millisecond)
				res[i], errs[i] = svc.Submit(ctx, p, task.ID, key(i), tasks.SubmitInput{Answer: yes}, auth.Meta{})
			}()
		}
		wg.Wait()
		return res, errs, task.ID
	}
	count := func(task uuid.UUID) (n int) {
		_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT count(*) FROM submissions WHERE assignment_id = $1 AND status = 'submitted'`, task).Scan(&n)
		})
		return n
	}

	res, errs, task := race(people[0], func(int) string { return "same-key-000001" })
	for i, err := range errs {
		if err != nil {
			t.Fatalf("same key, call %d: %v", i, err)
		}
		if res[i].SubmissionID != res[0].SubmissionID {
			t.Fatalf("same key gave different submissions: %v", res)
		}
	}
	if n := count(task); n != 1 {
		t.Fatalf("same key: %d submissions", n)
	}

	res, errs, task = race(people[1], func(i int) string { return fmt.Sprintf("other-key-%06d", i) })
	ok := 0
	for i, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, tasks.ErrAlreadySubmitted):
		default:
			t.Fatalf("different keys, call %d: %v (%+v)", i, err, res[i])
		}
	}
	if ok != 1 || count(task) != 1 {
		t.Fatalf("different keys: %d accepted, %d submissions", ok, count(task))
	}
}

// TestExpiryIsLocalAndSwept: asking for work expires only the asker's own
// lapsed reservation; the background sweep handles everyone else's.
func TestExpiryIsLocalAndSwept(t *testing.T) {
	pool, ctx := setup(t)
	project := bigProject(t, pool, 2, 10, 0, 0)
	people := crowd(t, pool, 2)
	svc := tasks.NewService(pool)
	a, err := svc.Next(ctx, people[0], project)
	if err != nil {
		t.Fatal(err)
	}
	_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE assignments SET reserved_at = now() - interval '2 hours', expires_at = now() - interval '1 hour' WHERE id = $1`, a.ID)
		return err
	})
	status := func() (s string) {
		_ = pool.InScope(ctx, db.Staff(), func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, `SELECT status FROM assignments WHERE id = $1`, a.ID).Scan(&s)
		})
		return s
	}
	if _, err := svc.Next(ctx, people[1], project); err != nil {
		t.Fatal(err)
	}
	if s := status(); s != "reserved" {
		t.Fatalf("someone else's request touched the lapsed row: %s", s)
	}
	if n, err := svc.SweepExpired(ctx); err != nil || n != 1 {
		t.Fatalf("sweep: %d %v", n, err)
	}
	if s := status(); s != "expired" {
		t.Fatalf("after sweep: %s", s)
	}
}

// TestLockTimeoutIsBusy: a request stuck behind a held lock gives up after
// the lock timeout with a retryable "busy" error instead of waiting forever.
func TestLockTimeoutIsBusy(t *testing.T) {
	pool, ctx := setup(t)
	project := bigProject(t, pool, 1, 5, 0, 0)
	p := crowd(t, pool, 1)[0]
	hold, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Rollback(ctx)
	if _, err := hold.Exec(ctx, `SELECT 1 FROM contributor_profiles WHERE id = $1 FOR UPDATE`, p.ContributorID); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = tasks.NewService(pool).Next(ctx, p, project)
	if !tasks.IsBusy(err) {
		t.Fatalf("want a busy error, got %v", err)
	}
	if d := time.Since(start); d < 4*time.Second || d > 15*time.Second {
		t.Fatalf("gave up after %s, want about %s", d, db.LockTimeout)
	}
}
