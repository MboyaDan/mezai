package delivery

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// The public sample delivery (Stage 21b, D-162). Fictional items and answers
// go through assemble, the same code as a real export, so the files on the
// site's Proof page have exactly the shape a client receives. Every name says
// "(Sample)" and nothing here describes real work. cmd/samplepackage writes
// the files; TestSamplePackageCommitted fails if the committed copies drift.

// SampleFiles names the sample package's files by kind (manifest, jsonl, csv).
type SampleFiles map[string][]byte

var sampleNS = uuid.MustParse("6f1c2a52-3b0e-4d8e-9a57-5a6d1f0c0b21")

func sampleID(name string) uuid.UUID { return uuid.NewSHA1(sampleNS, []byte(name)) }

type sampleItem struct {
	ext, text, final string
	answers          [3]string
	by               [3]string
	reviewed         bool
}

const (
	lOrder   = "order_status"
	lPayment = "payment"
	lReturns = "returns"
	lAccount = "account"
	lOther   = "other"
)

// Everyday messages to a fictional online shop, written for this sample.
var sampleItems = []sampleItem{
	{"msg-0001", "Oda yangu bado haijafika, ni siku ya tatu sasa.", lOrder, [3]string{lOrder, lOrder, lOrder}, [3]string{"C-0104", "C-0117", "C-0123"}, false},
	{"msg-0002", "I paid twice for the same order, please refund one.", lPayment, [3]string{lPayment, lPayment, lPayment}, [3]string{"C-0117", "C-0131", "C-0142"}, false},
	{"msg-0003", "Nataka kurudisha viatu, size ni ndogo.", lReturns, [3]string{lReturns, lReturns, lReturns}, [3]string{"C-0104", "C-0123", "C-0142"}, false},
	{"msg-0004", "Siwezi kuingia kwa akaunti yangu, nimesahau password.", lAccount, [3]string{lAccount, lAccount, lAccount}, [3]string{"C-0123", "C-0131", "C-0104"}, false},
	{"msg-0005", "Mzigo umefika lakini box ilikuwa imefunguliwa.", lReturns, [3]string{lReturns, lOrder, lOther}, [3]string{"C-0142", "C-0104", "C-0117"}, true},
	{"msg-0006", "Nimelipa na M-Pesa lakini oda inasema haijalipwa.", lPayment, [3]string{lPayment, lPayment, lOrder}, [3]string{"C-0131", "C-0117", "C-0123"}, false},
	{"msg-0007", "Do you deliver to Kisumu on weekends?", lOther, [3]string{lOther, lOther, lOrder}, [3]string{"C-0104", "C-0142", "C-0131"}, false},
	{"msg-0008", "App inaniambia namba yangu tayari imetumika.", lAccount, [3]string{lAccount, lAccount, lAccount}, [3]string{"C-0117", "C-0123", "C-0142"}, false},
	{"msg-0009", "Nimebadilisha mawazo, naweza cancel kabla haijatumwa?", lOrder, [3]string{lReturns, lOrder, lOther}, [3]string{"C-0131", "C-0104", "C-0123"}, true},
	{"msg-0010", "Asante, kila kitu kimefika poa.", lOther, [3]string{lOther, lOther, lOther}, [3]string{"C-0142", "C-0117", "C-0104"}, false},
	{"msg-0011", "The receipt shows the wrong amount.", lPayment, [3]string{lPayment, lPayment, lPayment}, [3]string{"C-0123", "C-0142", "C-0131"}, false},
	{"msg-0012", "Rider alipiga simu lakini sikuwa nyumbani, atarudi lini?", lOrder, [3]string{lOrder, lOrder, lOrder}, [3]string{"C-0104", "C-0131", "C-0117"}, false},
}

func labelsJSON(l string) []byte {
	b, _ := json.Marshal(map[string][]string{"labels": {l}})
	return b
}

// SamplePackage builds the sample delivery's manifest, JSONL and CSV.
func SamplePackage() (SampleFiles, error) {
	cfg := quality.ProjectConfig{Labels: []string{lOrder, lPayment, lReturns, lAccount, lOther}}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	project := store.LockProjectForExportRow{ID: sampleID("project"), OrgID: sampleID("org"),
		Name: "Customer message intent (Sample)", TaskType: quality.Classification, Config: cfgJSON,
		OverlapK: 3, OrgName: "Sample Client Ltd (Sample)"}

	start := time.Date(2026, 11, 16, 7, 30, 0, 0, time.UTC)
	var rows []store.ExportItemsRow
	var answers []store.ExportAnswersRow
	var adjudicated int32
	for i, it := range sampleItems {
		content, err := json.Marshal(map[string]string{"text": it.text})
		if err != nil {
			return nil, err
		}
		ext := it.ext
		id := sampleID(it.ext)
		at := start.Add(time.Duration(i) * 47 * time.Minute)
		source := "consensus"
		if it.reviewed {
			source, adjudicated = "review", adjudicated+1
		}
		var agreeing int32
		for _, a := range it.answers {
			if a == it.final {
				agreeing++
			}
			answers = append(answers, store.ExportAnswersRow{ItemID: id, Answer: labelsJSON(a)})
		}
		rows = append(rows, store.ExportItemsRow{ID: id, ExternalID: &ext, Content: content, FinalisedAt: &at,
			Revision: 1, Answer: labelsJSON(it.final), Source: source, Rated: 3, Agreeing: agreeing,
			Contributors: it.by[:]})
	}
	n := int32(len(rows))
	// One more item was flagged as unreadable and rejected at review, so it
	// counts in the totals but is not delivered.
	counts := store.ExportItemCountsRow{Total: n + 1, Final: n, Consensus: n - adjudicated, Adjudicated: adjudicated,
		Rejected: 1, Completed: n + 1, Routed: adjudicated + 1}
	gold := []store.ExportGoldByContributorRow{
		{PublicCode: "C-0104", Total: 8, Correct: 8},
		{PublicCode: "C-0117", Total: 8, Correct: 7},
		{PublicCode: "C-0123", Total: 7, Correct: 6},
		{PublicCode: "C-0131", Total: 7, Correct: 5},
		{PublicCode: "C-0142", Total: 8, Correct: 7},
	}
	reasons := []store.ExportRouteReasonsRow{{Reason: "flagged", N: 1}, {Reason: "no_majority", N: adjudicated}}

	pk, err := assemble(exportInput{Project: project, Config: cfg, Counts: counts, Rows: rows, Answers: answers,
		Gold: gold, Reasons: reasons, Version: 1, Now: time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		return nil, err
	}
	return SampleFiles{"manifest": append(pk.ManifestJSON, '\n'), "jsonl": pk.JSONL, "csv": pk.CSV}, nil
}

// SampleFileName is the published name of one sample file.
func SampleFileName(kind string) string {
	return FileName("Customer message intent (Sample)", 1, kind)
}
