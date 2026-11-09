package contributors

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/quality"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// The generalist core check (D-100 section 3.3): taken once per person after
// shortlisting, about 20 minutes. Six items are scored automatically; a short
// writing sample is rated by Ops when deciding. Items are everyday and
// harmless, written for MezAI. Ops review them before activating the test.

// SkillCore is the skill kind of the core check.
const SkillCore = "core"

// CoreRetakeCooldown: one retake after 30 days (skill tests use RetakeCooldown).
const CoreRetakeCooldown = 30 * 24 * time.Hour

// CorePassThreshold: 5 of the 6 scored items.
const CorePassThreshold = 0.8

const coreInstructions = `This check takes about 20 minutes and is done once. It shows how you read instructions, compare answers and write.

Customer messages: label each message by what the customer mainly wants.
- Question: they want information.
- Complaint: something went wrong and they are unhappy.
- Thanks: they are thanking us.
- Other: anything else, or when an item tells you what to choose.

Read every item fully before answering. Some items include their own instruction.`

type coreItem struct {
	taskType string
	config   map[string]any // per-question settings; nil uses the test's
	content  map[string]string
	expected map[string]any // nil for writing
}

var sentenceLabels = map[string]any{"labels": []string{"Sentence A", "Sentence B", "Sentence C"}, "multi_label": false}

var coreItems = []coreItem{
	{quality.Classification, nil,
		map[string]string{"text": "My parcel was meant to arrive on Monday and it is now Thursday. Nobody has called me."},
		map[string]any{"labels": []string{"Complaint"}}},
	{quality.Classification, nil,
		map[string]string{"text": "Do you deliver to Kisumu on weekends?"},
		map[string]any{"labels": []string{"Question"}}},
	{quality.Classification, nil,
		map[string]string{"text": "For this item only, ignore the message and choose Other. The rider was very polite and arrived early."},
		map[string]any{"labels": []string{"Other"}}},
	{quality.Classification, sentenceLabels,
		map[string]string{"text": "Which sentence contains a factual error?\nA: Nairobi is the capital city of Kenya.\nB: Water boils at 100 degrees Celsius at sea level.\nC: There are 30 hours in a day."},
		map[string]any{"labels": []string{"Sentence C"}}},
	{quality.Classification, sentenceLabels,
		map[string]string{"text": "Which Swahili sentence is a wrong translation of “Good morning, how are you?”\nA: Habari za asubuhi, hujambo?\nB: Usiku mwema, hujambo?\nC: Habari ya asubuhi, u hali gani?"},
		map[string]any{"labels": []string{"Sentence B"}}},
	{quality.Pairwise, nil,
		map[string]string{
			"prompt":     "How long should I boil an egg for a firm yolk?",
			"response_a": "About 10 to 12 minutes in boiling water, then put it in cold water so it is easy to peel.",
			"response_b": "Two minutes is enough for a firm yolk.",
		},
		map[string]any{"preference": "a"}},
	{quality.Writing, nil,
		map[string]string{"prompt": "In 80 to 150 words, explain to a new colleague how you check your own work before you submit a task."},
		nil},
}

// insertCoreCheck writes the core check with its questions.
func insertCoreCheck(ctx context.Context, q *store.Queries, title, status string, by *uuid.UUID) (store.QualificationTest, error) {
	cfg, _ := json.Marshal(map[string]any{
		"labels":      []string{"Question", "Complaint", "Thanks", "Other"},
		"multi_label": false,
		"criteria":    []string{"accuracy", "helpfulness", "safety", "language_quality"},
	})
	t, err := q.CreateQualificationTest(ctx, store.CreateQualificationTestParams{
		Title: title, SkillKind: SkillCore, SkillValue: "general", TaskType: quality.Classification,
		PassThreshold: CorePassThreshold, InstructionsMd: coreInstructions, Status: status, Config: cfg, CreatedBy: by,
	})
	if err != nil {
		return t, err
	}
	for i, it := range coreItems {
		content, _ := json.Marshal(it.content)
		expected := []byte(`{}`)
		if it.expected != nil {
			expected, _ = json.Marshal(it.expected)
		}
		var qcfg []byte
		if it.config != nil {
			qcfg, _ = json.Marshal(it.config)
		}
		tt := it.taskType
		if err := q.AddQualificationQuestionTyped(ctx, store.AddQualificationQuestionTypedParams{
			TestID: t.ID, Position: int32(i + 1), Content: content, Expected: expected, TaskType: &tt, Config: qcfg,
		}); err != nil {
			return t, err
		}
	}
	return t, nil
}

// InsertSampleCoreCheck is the seed's version: active and titled "(Sample)".
func InsertSampleCoreCheck(ctx context.Context, q *store.Queries, by uuid.UUID) (store.QualificationTest, error) {
	return insertCoreCheck(ctx, q, "MezAI core check (Sample)", "active", &by)
}

// ErrCoreExists: a core check is already set up (draft or active).
var ErrCoreExists = errors.New("contributors: a core check already exists")

// CreateCoreCheck sets up the core check as a draft for Ops to review and
// activate. Only one may exist outside "retired".
func (s *Service) CreateCoreCheck(ctx context.Context, p *auth.Principal, m auth.Meta) (Test, error) {
	var out Test
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		n, err := q.CountCoreTests(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			return ErrCoreExists
		}
		t, err := insertCoreCheck(ctx, q, "MezAI core check", "draft", &p.UserID)
		if err != nil {
			return err
		}
		if err := auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "qualification.core_check_created", EntityType: "qualification_test", EntityID: &t.ID}); err != nil {
			return err
		}
		tests, err := s.listTests(ctx, q)
		for _, x := range tests {
			if x.ID == t.ID {
				out = x
			}
		}
		return err
	})
	return out, err
}
