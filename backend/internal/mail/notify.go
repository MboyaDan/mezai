package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Contributor notifications (Stage 16). Links in them are paths; the worker
// puts the site origin in front when it sends (Worker.Origin).
const (
	TemplateQualificationResult = "qualification_result" // always sent
	TemplateProjectAvailable    = "project_available"    // optional
	TemplateFeedbackReceived    = "feedback_received"    // optional, at most once a day
	TemplatePayoutSent          = "payout_sent"          // always sent
)

// Notice is one notification about one person.
type Notice struct {
	// Exactly one of User and Contributor names the person.
	User        *uuid.UUID
	Contributor *uuid.UUID
	Template    string
	// Key makes the notice go out once: a repeated key is ignored.
	Key  string
	Data map[string]string
}

// Notify queues a notice inside the caller's transaction, unless the person
// cannot be reached (disabled, unverified address) or switched this kind
// off. It reports whether a message was queued (or already had been).
func Notify(ctx context.Context, q *store.Queries, n Notice) (bool, error) {
	var t store.NotifyTargetByUserRow
	var err error
	switch {
	case n.User != nil:
		t, err = q.NotifyTargetByUser(ctx, *n.User)
	case n.Contributor != nil:
		var r store.NotifyTargetByContributorRow
		r, err = q.NotifyTargetByContributor(ctx, *n.Contributor)
		t = store.NotifyTargetByUserRow(r)
	default:
		return false, errors.New("mail: notice names nobody")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !t.Reachable || !Wanted(n.Template, t.ProjectAvailable, t.FeedbackReceived) {
		return false, nil
	}
	data := map[string]string{"Name": t.DisplayName}
	for k, v := range n.Data {
		data[k] = v
	}
	b, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	key := n.Template + ":" + n.Key
	if err := q.EnqueueUserEmail(ctx, store.EnqueueUserEmailParams{ToAddress: t.Email, Template: n.Template, Data: b,
		UserID: &t.ID, DedupeKey: &key}); err != nil {
		return false, err
	}
	return true, nil
}

// Wanted applies a person's choices: only optional kinds can be switched off.
func Wanted(template string, projectAvailable, feedbackReceived bool) bool {
	switch template {
	case TemplateProjectAvailable:
		return projectAvailable
	case TemplateFeedbackReceived:
		return feedbackReceived
	default:
		return true
	}
}

// Money formats integer cents as "KES 1,250.00" for an email.
func Money(cents int64, currency string) string {
	neg := cents < 0
	if neg {
		cents = -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s %s%s.%02d", currency, sign, b.String(), cents%100)
}
