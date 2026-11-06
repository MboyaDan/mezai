package imports

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/quality"
)

var labels = quality.ProjectConfig{Labels: []string{"Billing", "Network", "Other"}}

func spec(kind, format, taskType string) Spec {
	cfg := labels
	if taskType == quality.Pairwise {
		cfg = quality.ProjectConfig{Criteria: []string{"accuracy"}}
	}
	return Spec{Kind: kind, Format: format, TaskType: taskType, Config: cfg, Taken: map[string]bool{"taken-1": true}}
}

func TestCSVItemsValid(t *testing.T) {
	data := "\xef\xbb\xbfexternal_id,text\nm-1,\"Line one, with a comma\"\nm-2,Second\n\n,No ID is fine\n"
	r := Validate(spec(KindItems, CSV, quality.Classification), []byte(data))
	if !r.Valid() || r.TotalRows != 3 || len(r.Rows) != 3 {
		t.Fatalf("report = %+v", r)
	}
	if r.Rows[0].Number != 2 || *r.Rows[0].ExternalID != "m-1" || r.Rows[2].ExternalID != nil {
		t.Fatalf("rows = %+v", r.Rows)
	}
	var c map[string]string
	_ = json.Unmarshal(r.Rows[0].Content, &c)
	if c["text"] != "Line one, with a comma" {
		t.Fatalf("content = %v", c)
	}
}

func TestCSVRowErrorsAreLocated(t *testing.T) {
	data := "external_id,text\nm-1,ok\nm-1,repeat\ntaken-1,used\nm-4,\nm-5,a,extra\n"
	r := Validate(spec(KindItems, CSV, quality.Classification), []byte(data))
	if r.Valid() || r.TotalRows != 5 || r.ErrorRows != 4 || len(r.Rows) != 1 {
		t.Fatalf("report = %+v", r)
	}
	want := []RowError{
		{3, "external_id", "repeats row 2"},
		{4, "external_id", "is already used in this project"},
		{5, "text", "is required"},
		{6, "", "has 3 cells; the header has 2"},
	}
	for i, w := range want {
		if r.Errors[i] != w {
			t.Errorf("error %d = %+v, want %+v", i, r.Errors[i], w)
		}
	}
}

func TestCSVHeaderProblems(t *testing.T) {
	for name, data := range map[string]string{
		"unknown column": "text,phone\nhi,0700\n",
		"missing column": "external_id\nm-1\n",
		"duplicate":      "text,text\na,b\n",
		"empty":          "text\n",
	} {
		r := Validate(spec(KindItems, CSV, quality.Classification), []byte(data))
		if r.Valid() || r.Errors[0].Row != 1 {
			t.Errorf("%s: report = %+v", name, r)
		}
	}
}

func TestNotUTF8(t *testing.T) {
	r := Validate(spec(KindItems, CSV, quality.Classification), []byte("text\n\xff\xfe\n"))
	if r.Valid() || r.Errors[0].Field != "file" {
		t.Fatalf("report = %+v", r)
	}
}

func TestJSONLGold(t *testing.T) {
	data := `{"external_id":"g-1","text":"Bundle ended early","expected":{"labels":["Billing"]}}
{"text":"Calls drop","expected":{"labels":["Weather"]}}
{"text":"x","expected":{"labels":["Billing","Other"]}}
{"text":"x","phone":"0700000000","expected":{"labels":["Other"]}}
not json
`
	r := Validate(spec(KindGold, JSONL, quality.Classification), []byte(data))
	if r.TotalRows != 5 || r.ErrorRows != 4 || len(r.Rows) != 1 {
		t.Fatalf("report = %+v", r)
	}
	if string(r.Rows[0].Expected) != `{"labels":["Billing"]}` || string(r.Rows[0].Tolerance) != `{}` {
		t.Fatalf("gold row = %s %s", r.Rows[0].Expected, r.Rows[0].Tolerance)
	}
	for _, e := range r.Errors {
		if strings.Contains(e.Message, "0700000000") || strings.Contains(e.Message, "Calls drop") {
			t.Errorf("error quotes content: %+v", e)
		}
	}
}

func TestCSVTranslationGold(t *testing.T) {
	data := "source,translation,source_lang,target_lang,expected_adequacy,expected_fluency,tolerance_adequacy\n" +
		"Good morning,Habari ya asubuhi,en,sw,5,5,0\n" +
		"Thank you,Asante,en,en,5,5,\n" +
		"Thank you,Asante,en,sw,five,5,\n"
	r := Validate(spec(KindGold, CSV, quality.TranslationReview), []byte(data))
	if r.TotalRows != 3 || r.ErrorRows != 2 {
		t.Fatalf("report = %+v", r)
	}
	if string(r.Rows[0].Tolerance) != `{"adequacy":0}` {
		t.Fatalf("tolerance = %s", r.Rows[0].Tolerance)
	}
	if r.Errors[0].Field != "target_lang" || r.Errors[1].Field != "expected_adequacy" {
		t.Fatalf("errors = %+v", r.Errors)
	}
}

func TestPairwiseGoldCSV(t *testing.T) {
	data := "prompt,response_a,response_b,expected_preference\nQ,A,B,A\nQ,A,B,maybe\n"
	r := Validate(spec(KindGold, CSV, quality.Pairwise), []byte(data))
	if r.TotalRows != 2 || r.ErrorRows != 1 || string(r.Rows[0].Expected) != `{"preference":"a"}` {
		t.Fatalf("report = %+v", r)
	}
}

func TestRowLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("text\n")
	for range MaxRows + 2 {
		b.WriteString("x\n")
	}
	r := Validate(spec(KindItems, CSV, quality.Classification), []byte(b.String()))
	if r.Valid() || r.ErrorRows != 2 || len(r.Errors) != 1 {
		t.Fatalf("total=%d errors=%d reported=%d", r.TotalRows, r.ErrorRows, len(r.Errors))
	}
}

func TestFormatOf(t *testing.T) {
	for name, want := range map[string]string{"a.CSV": CSV, "b.jsonl": JSONL, "c.ndjson": JSONL, "d.xlsx": ""} {
		if got, _ := FormatOf(name); got != want {
			t.Errorf("%s = %q", name, got)
		}
	}
}
