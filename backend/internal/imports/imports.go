// Package imports parses and validates item and gold files (CSV or JSONL)
// against a project's task type (brief 6.3, AC-05). It is pure: the caller
// stores the file, records the report and commits valid rows in one
// transaction, so an invalid file imports nothing.
package imports

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/MboyaDan/mezai/backend/internal/quality"
)

// Limits for one file.
const (
	MaxBytes      = 10 << 20 // 10 MB
	MaxRows       = 20000
	MaxExternalID = 200
	// MaxReported is how many row errors a report keeps; the counts are always complete.
	MaxReported = 1000
)

// Kinds and formats, as stored in import_jobs.
const (
	KindItems = "items"
	KindGold  = "gold"
	CSV       = "csv"
	JSONL     = "jsonl"
)

// Row is one valid row, ready to insert.
type Row struct {
	Number     int // file row (CSV: header is row 1) or line (JSONL)
	ExternalID *string
	Content    json.RawMessage
	Expected   json.RawMessage // gold only
	Tolerance  json.RawMessage // gold only
}

// RowError is one problem, located for the person fixing the file. Messages
// name fields and rules, never the cell's content.
type RowError struct {
	Row     int    `json:"row"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Report is the outcome of validating a whole file.
type Report struct {
	Rows      []Row      // valid rows, in file order
	Errors    []RowError // at most MaxReported
	TotalRows int
	ErrorRows int
}

// Valid reports whether the file can be committed.
func (r *Report) Valid() bool { return r.ErrorRows == 0 && r.TotalRows > 0 }

func (r *Report) fail(row int, field, msg string) {
	if len(r.Errors) < MaxReported {
		r.Errors = append(r.Errors, RowError{Row: row, Field: field, Message: msg})
	}
}

// Spec says what the file must hold.
type Spec struct {
	Kind     string
	Format   string
	TaskType string
	Config   quality.ProjectConfig
	// Taken holds external IDs already used in the project.
	Taken map[string]bool
}

// FormatOf picks the format from a file name.
func FormatOf(name string) (string, bool) {
	n := strings.ToLower(name)
	switch {
	case strings.HasSuffix(n, ".csv"):
		return CSV, true
	case strings.HasSuffix(n, ".jsonl"), strings.HasSuffix(n, ".ndjson"):
		return JSONL, true
	}
	return "", false
}

// Columns lists the CSV header a spec expects (optional ones included).
func Columns(kind, taskType string) []string {
	cols := append([]string{"external_id"}, quality.ContentFields(taskType)...)
	if kind != KindGold {
		return cols
	}
	switch taskType {
	case quality.Classification:
		return append(cols, "expected_labels")
	case quality.Pairwise:
		return append(cols, "expected_preference")
	case quality.TranslationReview:
		return append(cols, "expected_adequacy", "expected_fluency", "expected_corrected", "tolerance_adequacy", "tolerance_fluency")
	}
	return cols
}

// Validate reads the whole file and checks every row. A file-level problem
// (encoding, header, size) is reported as an error on row 1 (CSV) or line 1.
func Validate(spec Spec, data []byte) *Report {
	r := &Report{}
	if len(data) > MaxBytes {
		r.fail(1, "file", fmt.Sprintf("the file is larger than %d MB", MaxBytes>>20))
		r.ErrorRows = 1
		return r
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // spreadsheet BOM
	if !utf8.Valid(data) {
		r.fail(1, "file", "the file is not UTF-8 text; save it as UTF-8 and upload again")
		r.ErrorRows = 1
		return r
	}
	switch spec.Format {
	case CSV:
		validateCSV(spec, data, r)
	case JSONL:
		validateJSONL(spec, data, r)
	default:
		r.fail(1, "file", "use a .csv or .jsonl file")
		r.ErrorRows = 1
	}
	if r.TotalRows == 0 && r.ErrorRows == 0 {
		r.fail(1, "file", "the file has no rows")
		r.ErrorRows = 1
	}
	return r
}

// row is one parsed row before checks: item fields, ID and gold parts.
type row struct {
	number    int
	id        string
	fields    map[string]string
	expected  json.RawMessage
	tolerance json.RawMessage
}

// check validates one parsed row and appends it or its first error.
func check(spec Spec, rw row, r *Report, seen map[string]int) {
	r.TotalRows++
	if r.TotalRows > MaxRows {
		if r.TotalRows == MaxRows+1 {
			r.fail(rw.number, "file", fmt.Sprintf("the file has more than %d rows; split it", MaxRows))
		}
		r.ErrorRows++
		return
	}
	failed := func(field, msg string) {
		r.fail(rw.number, field, msg)
		r.ErrorRows++
	}
	var ext *string
	if id := strings.TrimSpace(rw.id); id != "" {
		switch {
		case utf8.RuneCountInString(id) > MaxExternalID:
			failed("external_id", fmt.Sprintf("use at most %d characters", MaxExternalID))
			return
		case seen[id] > 0:
			failed("external_id", fmt.Sprintf("repeats row %d", seen[id]))
			return
		case spec.Taken[id]:
			failed("external_id", "is already used in this project")
			return
		}
		seen[id] = rw.number
		ext = &id
	}
	content, err := quality.ValidateContent(spec.TaskType, rw.fields)
	if err != nil {
		failed(fieldOf(err), messageOf(err))
		return
	}
	out := Row{Number: rw.number, ExternalID: ext, Content: content}
	if spec.Kind == KindGold {
		if out.Expected, err = quality.ValidateExpected(spec.TaskType, spec.Config, rw.expected); err != nil {
			failed(fieldOf(err), messageOf(err))
			return
		}
		if out.Tolerance, err = quality.ValidateTolerance(spec.TaskType, rw.tolerance); err != nil {
			failed(fieldOf(err), messageOf(err))
			return
		}
	}
	r.Rows = append(r.Rows, out)
}

func fieldOf(err error) string {
	var iv *quality.Invalid
	if errors.As(err, &iv) {
		return iv.Field
	}
	return ""
}

func messageOf(err error) string {
	var iv *quality.Invalid
	if errors.As(err, &iv) {
		return iv.Message
	}
	return "invalid value"
}

func validateCSV(spec Spec, data []byte, r *Report) {
	cr := csv.NewReader(bytes.NewReader(data))
	cr.FieldsPerRecord = -1 // reported per row below
	header, err := cr.Read()
	if err != nil {
		r.fail(1, "file", "the first row must be a header")
		r.ErrorRows = 1
		return
	}
	allowed := Columns(spec.Kind, spec.TaskType)
	idx := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		if !slices.Contains(allowed, h) {
			r.fail(1, h, "unknown column; expected "+strings.Join(allowed, ", "))
			r.ErrorRows = 1
			return
		}
		if _, dup := idx[h]; dup {
			r.fail(1, h, "column appears twice")
			r.ErrorRows = 1
			return
		}
		idx[h] = i
	}
	for _, need := range quality.ContentFields(spec.TaskType) {
		if _, ok := idx[need]; !ok {
			r.fail(1, need, "missing column")
			r.ErrorRows = 1
			return
		}
	}
	seen := map[string]int{}
	for n := 2; ; n++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			// A broken quote breaks every row after it; stop here.
			r.TotalRows++
			r.ErrorRows++
			r.fail(n, "file", "the CSV cannot be read from this row on (check quotes)")
			return
		}
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue // blank line
		}
		if len(rec) != len(header) {
			r.TotalRows++
			r.ErrorRows++
			r.fail(n, "", fmt.Sprintf("has %d cells; the header has %d", len(rec), len(header)))
			continue
		}
		cell := func(col string) string {
			if i, ok := idx[col]; ok {
				return rec[i]
			}
			return ""
		}
		rw := row{number: n, id: cell("external_id"), fields: map[string]string{}}
		for _, f := range quality.ContentFields(spec.TaskType) {
			rw.fields[f] = cell(f)
		}
		if spec.Kind == KindGold {
			var perr error
			rw.expected, rw.tolerance, perr = goldFromCells(spec.TaskType, cell)
			if perr != nil {
				r.TotalRows++
				r.ErrorRows++
				r.fail(n, fieldOf(perr), messageOf(perr))
				continue
			}
		}
		check(spec, rw, r, seen)
	}
}

// goldFromCells turns expected_* and tolerance_* cells into JSON.
func goldFromCells(taskType string, cell func(string) string) (json.RawMessage, json.RawMessage, error) {
	num := func(col string) (*int, error) {
		v := strings.TrimSpace(cell(col))
		if v == "" {
			return nil, nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, &quality.Invalid{Field: col, Message: "use a whole number"}
		}
		return &n, nil
	}
	exp, tol := map[string]any{}, map[string]any{}
	switch taskType {
	case quality.Classification:
		var labels []string
		for _, l := range strings.Split(cell("expected_labels"), "|") {
			if l = strings.TrimSpace(l); l != "" {
				labels = append(labels, l)
			}
		}
		exp["labels"] = labels
	case quality.Pairwise:
		exp["preference"] = strings.ToLower(strings.TrimSpace(cell("expected_preference")))
	case quality.TranslationReview:
		for col, key := range map[string]string{"expected_adequacy": "adequacy", "expected_fluency": "fluency"} {
			n, err := num(col)
			if err != nil {
				return nil, nil, err
			}
			if n != nil {
				exp[key] = *n
			}
		}
		if c := cell("expected_corrected"); strings.TrimSpace(c) != "" {
			exp["corrected"] = c
		}
		for col, key := range map[string]string{"tolerance_adequacy": "adequacy", "tolerance_fluency": "fluency"} {
			n, err := num(col)
			if err != nil {
				return nil, nil, err
			}
			if n != nil {
				tol[key] = *n
			}
		}
	}
	e, _ := json.Marshal(exp)
	t, _ := json.Marshal(tol)
	return e, t, nil
}

func validateJSONL(spec Spec, data []byte, r *Report) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64<<10), MaxBytes)
	fields := quality.ContentFields(spec.TaskType)
	seen := map[string]int{}
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(line, &obj); err != nil || obj == nil {
			r.TotalRows++
			r.ErrorRows++
			r.fail(n, "", "is not a JSON object")
			continue
		}
		rw := row{number: n, fields: map[string]string{}}
		var problem *RowError
		for k, v := range obj {
			switch {
			case k == "external_id":
				if err := json.Unmarshal(v, &rw.id); err != nil {
					problem = &RowError{Row: n, Field: k, Message: "must be a string"}
				}
			case k == "expected" && spec.Kind == KindGold:
				rw.expected = v
			case k == "tolerance" && spec.Kind == KindGold:
				rw.tolerance = v
			case slices.Contains(fields, k):
				var s string
				if err := json.Unmarshal(v, &s); err != nil {
					problem = &RowError{Row: n, Field: k, Message: "must be a string"}
				}
				rw.fields[k] = s
			default:
				problem = &RowError{Row: n, Field: k, Message: "is not a field of this file; expected " + strings.Join(jsonlKeys(spec), ", ")}
			}
			if problem != nil {
				break
			}
		}
		if problem != nil {
			r.TotalRows++
			r.ErrorRows++
			r.fail(problem.Row, problem.Field, problem.Message)
			continue
		}
		check(spec, rw, r, seen)
	}
	if err := sc.Err(); err != nil {
		r.ErrorRows++
		r.fail(1, "file", "the file cannot be read as lines of JSON")
	}
}

func jsonlKeys(spec Spec) []string {
	keys := append([]string{"external_id"}, quality.ContentFields(spec.TaskType)...)
	if spec.Kind == KindGold {
		keys = append(keys, "expected", "tolerance")
	}
	return keys
}
