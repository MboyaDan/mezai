package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Record is one delivered item in the JSONL file (schema version 1).
type Record struct {
	ItemID       uuid.UUID       `json:"item_id"`
	ExternalID   *string         `json:"external_id"`
	Input        json.RawMessage `json:"input"`
	Answer       json.RawMessage `json:"answer"`
	DecidedBy    string          `json:"decided_by"` // consensus, review or correction
	Revision     int32           `json:"revision"`
	Agreement    *float64        `json:"agreement"` // share of answers agreeing with the final answer
	Answers      int             `json:"answers"`
	Contributors []string        `json:"contributors"` // public codes only (brief 6.3)
	FinalisedAt  *time.Time      `json:"finalised_at"`
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func ratio(n, d int32) *float64 {
	if d == 0 {
		return nil
	}
	v := round4(float64(n) / float64(d))
	return &v
}

func toRecord(r store.ExportItemsRow) Record {
	rec := Record{ItemID: r.ID, ExternalID: r.ExternalID, Input: r.Content, Answer: r.Answer, DecidedBy: r.Source,
		Revision: r.Revision, Answers: len(r.Contributors), Contributors: r.Contributors, FinalisedAt: r.FinalisedAt}
	if rec.Contributors == nil {
		rec.Contributors = []string{}
	}
	if r.Rated > 1 {
		rec.Agreement = ratio(r.Agreeing, r.Rated)
	}
	return rec
}

// guard stops a spreadsheet from running a cell as a formula (OWASP CSV
// injection): cells starting with = + - @ tab or carriage return get a
// leading apostrophe. JSONL is never changed.
func guard(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// buildFiles writes the JSONL and CSV files for the items, in the same order.
func buildFiles(taskType string, cfg quality.ProjectConfig, rows []store.ExportItemsRow) (jsonl, csvData []byte, err error) {
	var jb, cb bytes.Buffer
	enc := json.NewEncoder(&jb)
	enc.SetEscapeHTML(false)
	w := csv.NewWriter(&cb)
	fields := quality.ContentFields(taskType)
	header := append([]string{"item_id", "external_id"}, fields...)
	header = append(header, quality.ExportColumns(taskType, cfg)...)
	header = append(header, "decided_by", "revision", "agreement", "answers", "contributors", "finalised_at")
	if err := w.Write(header); err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		rec := toRecord(r)
		if err := enc.Encode(rec); err != nil {
			return nil, nil, err
		}
		var content map[string]string
		if err := json.Unmarshal(r.Content, &content); err != nil {
			return nil, nil, fmt.Errorf("delivery: item %s content: %w", r.ID, err)
		}
		vals, err := quality.ExportValues(taskType, cfg, r.Answer)
		if err != nil {
			return nil, nil, fmt.Errorf("delivery: item %s: %w", r.ID, err)
		}
		row := []string{r.ID.String(), ""}
		if r.ExternalID != nil {
			row[1] = *r.ExternalID
		}
		for _, f := range fields {
			row = append(row, content[f])
		}
		row = append(row, vals...)
		agreement, finalised := "", ""
		if rec.Agreement != nil {
			agreement = strconv.FormatFloat(*rec.Agreement, 'f', -1, 64)
		}
		if rec.FinalisedAt != nil {
			finalised = rec.FinalisedAt.UTC().Format(time.RFC3339)
		}
		row = append(row, rec.DecidedBy, strconv.Itoa(int(rec.Revision)), agreement, strconv.Itoa(rec.Answers),
			strings.Join(rec.Contributors, "|"), finalised)
		for i := range row {
			row[i] = guard(row[i])
		}
		if err := w.Write(row); err != nil {
			return nil, nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, nil, err
	}
	return jb.Bytes(), cb.Bytes(), nil
}

func checksum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

var notSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug makes a file-name-safe stem from the project name.
func slug(name string) string {
	s := strings.Trim(notSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		return "project"
	}
	return s
}

// FileName is the download name of a deliverable file.
func FileName(project string, version int32, file string) string {
	if file == "manifest" {
		return fmt.Sprintf("%s-v%d-manifest.json", slug(project), version)
	}
	return fmt.Sprintf("%s-v%d.%s", slug(project), version, file)
}

// Manifest is the delivery manifest (brief 8.5). It names contributors by
// public code only and holds no item content.
type Manifest struct {
	Schema        string          `json:"schema"`
	SchemaVersion int             `json:"schema_version"`
	Project       ManifestProject `json:"project"`
	Version       int32           `json:"version"`
	ExportedAt    time.Time       `json:"exported_at"`
	Items         ManifestItems   `json:"items"`
	GoldAccuracy  ManifestGold    `json:"gold_accuracy"`
	Agreement     []ManifestStat  `json:"agreement"`
	Review        ManifestReview  `json:"review"`
	Contributors  ManifestPeople  `json:"contributors"`
	Files         []ManifestFile  `json:"files"`
	Notes         []string        `json:"notes"`
}

type ManifestProject struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	TaskType     string    `json:"task_type"`
	Organisation string    `json:"organisation"`
	OverlapK     int16     `json:"overlap_k"`
}

// ManifestItems counts the project's regular items (gold excluded).
type ManifestItems struct {
	Total       int32 `json:"total"`
	Delivered   int32 `json:"delivered"`
	Consensus   int32 `json:"consensus"`
	Adjudicated int32 `json:"adjudicated"`
	Rejected    int32 `json:"rejected"`
	NotFinal    int32 `json:"not_final"`
	Complete    bool  `json:"complete"`
}

type ManifestGold struct {
	Results       int32              `json:"results"`
	Correct       int32              `json:"correct"`
	Accuracy      *float64           `json:"accuracy"`
	Distribution  ManifestSpread     `json:"distribution"`
	ByContributor []ContributorScore `json:"by_contributor"`
}

// ManifestSpread summarises per-contributor gold accuracy.
type ManifestSpread struct {
	Contributors int      `json:"contributors"`
	Min          *float64 `json:"min"`
	Median       *float64 `json:"median"`
	Max          *float64 `json:"max"`
}

type ContributorScore struct {
	Contributor string   `json:"contributor"`
	Results     int32    `json:"results"`
	Accuracy    *float64 `json:"accuracy"`
}

type ManifestStat struct {
	Statistic string  `json:"statistic"`
	Label     *string `json:"label"`
	Value     float64 `json:"value"`
	Band      *string `json:"band"`
	Items     int     `json:"items"`
	Raters    int     `json:"raters"`
}

type ManifestReview struct {
	Completed  int32            `json:"completed"`
	Routed     int32            `json:"routed"`
	ReviewRate *float64         `json:"review_rate"`
	Reasons    map[string]int32 `json:"reasons"`
}

type ManifestPeople struct {
	Count int      `json:"count"`
	IDs   []string `json:"ids"`
}

type ManifestFile struct {
	Name    string `json:"name"`
	Format  string `json:"format"`
	Records int    `json:"records"`
	Bytes   int    `json:"bytes"`
	SHA256  string `json:"sha256"`
}

var manifestNotes = []string{
	"Only final items are delivered. Gold (test) items and items rejected at review are never included.",
	"agreement on an item is the share of its answers that agree with the final answer; it is empty with fewer than two answers.",
	"decided_by is consensus (automatic), review (a reviewer decided) or correction (a later revision).",
	"Contributors are named by public code only.",
	"CSV: lists are joined with |; a cell beginning with = + - @ tab or carriage return is prefixed with ' so spreadsheets do not run it as a formula. The JSONL file is exact.",
}

// goldSummary turns per-contributor gold counts into the manifest's figures.
func goldSummary(rows []store.ExportGoldByContributorRow) ManifestGold {
	g := ManifestGold{ByContributor: []ContributorScore{}}
	accs := []float64{}
	for _, r := range rows {
		g.Results += r.Total
		g.Correct += r.Correct
		a := ratio(r.Correct, r.Total)
		g.ByContributor = append(g.ByContributor, ContributorScore{Contributor: r.PublicCode, Results: r.Total, Accuracy: a})
		if a != nil {
			accs = append(accs, *a)
		}
	}
	g.Accuracy = ratio(g.Correct, g.Results)
	g.Distribution.Contributors = len(accs)
	if n := len(accs); n > 0 {
		slices.Sort(accs)
		med := accs[n/2]
		if n%2 == 0 {
			med = round4((accs[n/2-1] + accs[n/2]) / 2)
		}
		g.Distribution.Min, g.Distribution.Median, g.Distribution.Max = &accs[0], &med, &accs[n-1]
	}
	return g
}

// agreementStats computes brief 8.2's figures over the delivered items that
// have exactly k answers (the same rater count on every item).
func agreementStats(taskType string, cfg quality.ProjectConfig, k int, rows []store.ExportAnswersRow) ([]ManifestStat, error) {
	items := [][]quality.Answer{}
	var cur uuid.UUID
	for _, r := range rows {
		var a quality.Answer
		if err := json.Unmarshal(r.Answer, &a); err != nil {
			return nil, err
		}
		if r.ItemID != cur || len(items) == 0 {
			items, cur = append(items, nil), r.ItemID
		}
		items[len(items)-1] = append(items[len(items)-1], a)
	}
	complete := items[:0]
	for _, it := range items {
		if len(it) == k {
			complete = append(complete, it)
		}
	}
	out := []ManifestStat{}
	for _, st := range quality.ProjectAgreement(taskType, cfg.Answers(), complete, k) {
		m := ManifestStat{Statistic: st.Statistic, Value: round4(st.Value), Items: st.Items, Raters: st.Raters}
		if st.Label != "" {
			l := st.Label
			m.Label = &l
		}
		if st.Statistic == "cohen_kappa" || st.Statistic == "fleiss_kappa" {
			b := quality.Band(st.Value)
			m.Band = &b
		}
		out = append(out, m)
	}
	return out, nil
}
