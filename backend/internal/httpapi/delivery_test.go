package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type deliverableFile struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	Format string `json:"format"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type deliverableVersion struct {
	Version     int     `json:"version"`
	Items       int     `json:"items"`
	PublishedAt *string `json:"published_at"`
	Manifest    struct {
		SchemaVersion int `json:"schema_version"`
		Items         struct {
			Total, Delivered, Consensus, Adjudicated, Rejected int
			NotFinal                                           int  `json:"not_final"`
			Complete                                           bool `json:"complete"`
		} `json:"items"`
		Agreement []struct {
			Statistic string  `json:"statistic"`
			Value     float64 `json:"value"`
			Items     int     `json:"items"`
		} `json:"agreement"`
		Review struct {
			Completed, Routed int
			ReviewRate        *float64 `json:"review_rate"`
		} `json:"review"`
		Contributors struct {
			Count int      `json:"count"`
			IDs   []string `json:"ids"`
		} `json:"contributors"`
		Files []struct {
			Name, Format string
			Records      int
			SHA256       string `json:"sha256"`
		} `json:"files"`
	} `json:"manifest"`
	Files []deliverableFile `json:"files"`
}

func (v deliverableVersion) file(file, format string) deliverableFile {
	for _, f := range v.Files {
		if f.File == file && f.Format == format {
			return f
		}
	}
	return deliverableFile{}
}

// fetch gets a signed link for the file and follows it.
func (e *env) fetch(s *session, f deliverableFile) (link string, w *bytes.Buffer, code int) {
	e.t.Helper()
	lw := e.do(http.MethodPost, "/v1/deliverables/"+f.ID+"/links", map[string]string{"file": f.File}, s, nil)
	if lw.Code != http.StatusOK {
		return "", nil, lw.Code
	}
	var l struct{ URL string }
	decode(e.t, lw, &l)
	link = strings.TrimPrefix(l.URL, "/api")
	dw := e.do(http.MethodGet, link, nil, s, nil)
	return link, dw.Body, dw.Code
}

func sha(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// TestDeliverableExport is AC-09's delivery half: the export holds only final
// regular items, with a manifest whose counts and checksums are right; the
// client sees it only once published; links are signed for one user; every
// download leaves a row; a changed file is never served.
func TestDeliverableExport(t *testing.T) {
	e := newEnv(t)
	project := e.reviewProject(4)
	_, c1 := e.login("contributor001@sample.invalid", pw)
	_, c2 := e.login("contributor002@sample.invalid", pw)
	_, ops := e.login("ops@sample.invalid", pw)
	_, rev := e.login("reviewer@sample.invalid", pw)
	_, client := e.login("client.admin@sample.invalid", pw)
	_, other := e.login("other.admin@sample.invalid", pw)

	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, ops, nil); w.Code != http.StatusConflict || errorCode(w) != "nothing_to_export" {
		t.Fatalf("export with nothing final: %d %s", w.Code, w.Body)
	}

	// Round 1: both agree on four items (consensus). Round 2: they disagree
	// on two more (review: one chosen, one rejected). One item is never
	// worked, and a gold item exists: neither may be delivered.
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"Yes"}})
	e.staff(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO items (org_id, project_id, external_id, content)
			SELECT org_id, id, x, '{"text":"=HYPERLINK(\"x\")"}' FROM projects, unnest(ARRAY['rv-a','rv-b']) x WHERE id = $1`, project)
		return err
	})
	e.workAll(c1, project, map[string]any{"labels": []string{"Yes"}})
	e.workAll(c2, project, map[string]any{"labels": []string{"No"}})
	var q reviewQueueResp
	decode(t, e.do(http.MethodGet, "/v1/review/items?project_id="+project, nil, ops, nil), &q)
	if len(q.Items) != 2 {
		t.Fatalf("queue: %+v", q)
	}
	if w := e.decide(ops, q.Items[0].ItemID, "dl-key-0001", map[string]any{"decision": "choose_submission", "submission_id": firstSubmission(e, q.Items[0].ItemID)}); w.Code != http.StatusOK {
		t.Fatalf("choose: %d %s", w.Code, w.Body)
	}
	if w := e.decide(ops, q.Items[1].ItemID, "dl-key-0002", map[string]any{"decision": "reject_item", "reason": "Not a customer message at all."}); w.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", w.Code, w.Body)
	}
	e.staff(func(tx pgx.Tx) error {
		ctx := context.Background()
		if _, err := tx.Exec(ctx, `INSERT INTO items (org_id, project_id, external_id, content)
			SELECT org_id, id, 'rv-pending', '{"text":"Sample sentence."}' FROM projects WHERE id = $1`, project); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `WITH i AS (
			  INSERT INTO items (org_id, project_id, external_id, content, status, finalised_at)
			  SELECT org_id, id, 'rv-gold', '{"text":"Sample gold."}', 'final', now() FROM projects WHERE id = $1 RETURNING id, org_id, project_id)
			INSERT INTO gold_items (org_id, project_id, item_id, expected) SELECT org_id, project_id, id, '{"labels":["Yes"]}' FROM i`, project)
		return err
	})

	// Only people who publish deliverables export.
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, rev, nil); w.Code != http.StatusForbidden {
		t.Fatalf("reviewer export: %d", w.Code)
	}
	w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, ops, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("export: %d %s", w.Code, w.Body)
	}
	var v deliverableVersion
	decode(t, w, &v)
	m := v.Manifest
	if v.Version != 1 || v.Items != 5 || m.SchemaVersion != 1 {
		t.Fatalf("version %d items %d schema %d", v.Version, v.Items, m.SchemaVersion)
	}
	if it := m.Items; it.Total != 7 || it.Delivered != 5 || it.Consensus != 4 || it.Adjudicated != 1 || it.Rejected != 1 || it.NotFinal != 1 || it.Complete {
		t.Fatalf("manifest items: %+v", it)
	}
	if m.Review.Completed != 6 || m.Review.Routed != 2 || m.Review.ReviewRate == nil || *m.Review.ReviewRate != 0.3333 {
		t.Fatalf("manifest review: %+v", m.Review)
	}
	if m.Contributors.Count != 2 || len(m.Contributors.IDs) != 2 || !strings.HasPrefix(m.Contributors.IDs[0], "C-") {
		t.Fatalf("manifest contributors: %+v", m.Contributors)
	}
	percent := false
	for _, a := range m.Agreement {
		if a.Statistic == "percent" {
			// 4 of 5 delivered items had both raters agree.
			percent = a.Items == 5 && a.Value == 0.8
		}
	}
	if !percent {
		t.Fatalf("manifest agreement: %+v", m.Agreement)
	}
	if len(v.Files) != 3 || len(m.Files) != 2 {
		t.Fatalf("files: %+v / %+v", v.Files, m.Files)
	}

	// The stored files match the manifest's checksums and hold only the
	// delivered items; CSV cells that look like formulas are neutralised.
	var keys []string
	e.staff(func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT file_key FROM deliverables WHERE project_id = $1 ORDER BY format DESC`, project)
		if err != nil {
			return err
		}
		keys, err = pgx.CollectRows(rows, pgx.RowTo[string])
		return err
	})
	jsonl, _ := e.files.Get(context.Background(), keys[0], 1<<20)
	csvData, _ := e.files.Get(context.Background(), keys[1], 1<<20)
	if sha(jsonl) != m.Files[0].SHA256 || sha(csvData) != m.Files[1].SHA256 || m.Files[0].Records != 5 {
		t.Fatalf("checksums or records do not match: %+v", m.Files)
	}
	ext := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(jsonl))
	for sc.Scan() {
		var rec struct {
			ExternalID   string `json:"external_id"`
			DecidedBy    string `json:"decided_by"`
			Contributors []string
		}
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		ext[rec.ExternalID] = true
		if len(rec.Contributors) != 2 {
			t.Errorf("%s: contributors %v", rec.ExternalID, rec.Contributors)
		}
	}
	if len(ext) != 5 || ext["rv-gold"] || ext["rv-pending"] {
		t.Fatalf("delivered %v", ext)
	}
	records, err := csv.NewReader(bytes.NewReader(csvData)).ReadAll()
	if err != nil || len(records) != 6 || records[0][2] != "text" || records[0][3] != "labels" {
		t.Fatalf("csv: %v %q", err, records)
	}
	guarded := false
	for _, r := range records {
		guarded = guarded || strings.HasPrefix(r[2], `'=HYPERLINK`)
	}
	if !guarded || !bytes.Contains(jsonl, []byte(`"=HYPERLINK(\"x\")"`)) {
		t.Fatal("formula cell not guarded in CSV, or JSONL changed")
	}

	// Unpublished: the client sees nothing and cannot link to it; another
	// organisation cannot even see the project.
	var list struct{ Versions []deliverableVersion }
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, client, nil), &list)
	if len(list.Versions) != 0 {
		t.Fatalf("client sees an unpublished version: %+v", list)
	}
	csvFile := v.file("data", "csv")
	if _, _, code := e.fetch(client, csvFile); code != http.StatusNotFound {
		t.Fatalf("client link to unpublished: %d", code)
	}
	if w := e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, other, nil); w.Code != http.StatusNotFound {
		t.Fatalf("other organisation lists: %d", w.Code)
	}

	pub := "/v1/projects/" + project + "/deliverables/1/publish"
	if w := e.do(http.MethodPost, pub, nil, ops, nil); w.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, pub, nil, ops, nil); errorCode(w) != "already_published" {
		t.Fatalf("publish twice: %d %s", w.Code, w.Body)
	}
	if w := e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables/9/publish", nil, ops, nil); w.Code != http.StatusNotFound {
		t.Fatalf("publish missing: %d", w.Code)
	}

	// Published: the client lists it and downloads through a signed link.
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, client, nil), &list)
	if len(list.Versions) != 1 || list.Versions[0].PublishedAt == nil || len(list.Versions[0].Files) != 3 {
		t.Fatalf("client list: %+v", list)
	}
	link, body, code := e.fetch(client, csvFile)
	if code != http.StatusOK || !bytes.Equal(body.Bytes(), csvData) {
		t.Fatalf("client download: %d", code)
	}
	if _, body, code := e.fetch(client, v.file("manifest", "json")); code != http.StatusOK || !bytes.Contains(body.Bytes(), []byte(`"mezai.delivery-manifest"`)) {
		t.Fatalf("manifest download: %d", code)
	}
	if n := e.count(`SELECT count(*) FROM deliverable_downloads WHERE user_id = (SELECT id FROM users WHERE email = 'client.admin@sample.invalid')`); n != 2 {
		t.Fatalf("%d download rows, want 2", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_log WHERE action = 'deliverable.downloaded'`); n != 2 {
		t.Fatalf("%d download audit rows, want 2", n)
	}

	// A link works only for the user it was signed for, and not when altered.
	if w := e.do(http.MethodGet, link, nil, ops, nil); w.Code != http.StatusForbidden {
		t.Fatalf("someone else's link: %d", w.Code)
	}
	if w := e.do(http.MethodGet, link, nil, other, nil); w.Code != http.StatusForbidden {
		t.Fatalf("other organisation with the link: %d", w.Code)
	}
	if w := e.do(http.MethodGet, strings.Replace(link, "file=data", "file=manifest", 1), nil, client, nil); w.Code != http.StatusForbidden {
		t.Fatalf("altered link: %d", w.Code)
	}
	if w := e.do(http.MethodGet, link, nil, nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", w.Code)
	}

	// A changed file is refused and nothing is recorded.
	e.files.Corrupt(keys[1], []byte("item_id\nforged\n"))
	if _, _, code := e.fetch(client, csvFile); code != http.StatusInternalServerError {
		t.Fatalf("changed file served: %d", code)
	}
	if n := e.count(`SELECT count(*) FROM deliverable_downloads`); n != 2 {
		t.Fatalf("%d download rows after a refused download, want 2", n)
	}

	// The next export is version 2, unpublished.
	w = e.do(http.MethodPost, "/v1/projects/"+project+"/deliverables", nil, ops, nil)
	decode(t, w, &v)
	if w.Code != http.StatusCreated || v.Version != 2 || v.PublishedAt != nil {
		t.Fatalf("second export: %d %+v", w.Code, v)
	}
	decode(t, e.do(http.MethodGet, "/v1/projects/"+project+"/deliverables", nil, ops, nil), &list)
	if len(list.Versions) != 2 || list.Versions[0].Version != 2 {
		t.Fatalf("ops list: %+v", list.Versions)
	}
}
