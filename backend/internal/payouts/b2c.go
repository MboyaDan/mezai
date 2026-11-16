package payouts

// M-Pesa B2C payouts (Stage 15a, brief 13, D-132 to D-135).
//
// An approved batch is disbursed: each pending line gets one transaction
// (never a second) and one send job. The send job moves the transaction to
// 'sending' and commits before it calls Daraja, so a crash or a timeout can
// never send the same payment twice. Answers:
//
//	accepted        -> wait for the result callback; a status query later as a safety net
//	refused         -> the line fails, nothing was paid, the amount stays owed
//	not accepted    -> back to 'queued', the job retries the same request (gateway refusals only)
//	unknown         -> status queries until M-Pesa says paid, failed or (after a while) not found
//
// Callbacks are stored with a verdict before they are processed, and
// processing is idempotent: a duplicate is a no-op, a contradiction is
// flagged for Ops and changes nothing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/jobs"
	"github.com/MboyaDan/mezai/backend/internal/mpesa"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// Job kinds on the Postgres queue.
const (
	JobB2CSend       = "mpesa.b2c_send"
	JobStatusQuery   = "mpesa.status_query"
	JobCallback      = "mpesa.callback"
	maxStatusQueries = 8
)

// Callback kinds (the route they arrived on).
const (
	CallbackB2CResult     = "b2c_result"
	CallbackB2CTimeout    = "b2c_timeout"
	CallbackStatusResult  = "status_result"
	CallbackStatusTimeout = "status_timeout"
)

var (
	ErrB2COff  = errors.New("payouts: M-Pesa B2C is not switched on")
	errSending = errors.New("payouts: transaction is being sent; try later")
)

// B2C connects the service to Daraja (or the fake).
type B2C struct {
	Client      mpesa.Client
	Environment string // fake, sandbox or production
}

// WithB2C switches automated payouts on.
func (s *Service) WithB2C(b *B2C) *Service {
	s.b2c = b
	return s
}

// B2CStatus says whether automated payouts are on and where they go.
type B2CStatus struct {
	Enabled     bool   `json:"enabled"`
	Environment string `json:"environment"`
}

// B2CInfo reports the B2C setting for the screens.
func (s *Service) B2CInfo() B2CStatus {
	if s.b2c == nil {
		return B2CStatus{Environment: "off"}
	}
	return B2CStatus{Enabled: true, Environment: s.b2c.Environment}
}

// OriginatorID is the payment's idempotency key at Safaricom: one per line.
func OriginatorID(lineID uuid.UUID) string {
	return "mz-" + strings.ReplaceAll(lineID.String(), "-", "")
}

func queryOriginatorID(paymentID string, n int) string {
	return fmt.Sprintf("%s-q%d", paymentID, n)
}

// paymentOriginator strips a status query's "-qN" suffix.
func paymentOriginator(id string) string {
	if i := strings.LastIndex(id, "-q"); i > 0 {
		if _, err := strconv.Atoi(id[i+2:]); err == nil {
			return id[:i]
		}
	}
	return id
}

// statusQueryDelay is the wait before status query n (1-based).
func statusQueryDelay(n int) time.Duration {
	steps := []time.Duration{2 * time.Minute, 5 * time.Minute, 15 * time.Minute, 30 * time.Minute,
		time.Hour, 2 * time.Hour, 4 * time.Hour, 8 * time.Hour}
	if n < 1 {
		n = 1
	}
	if n > len(steps) {
		return steps[len(steps)-1]
	}
	return steps[n-1]
}

func enqueue(ctx context.Context, q *store.Queries, kind, key string, payload any, at time.Time, attempts int32) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return q.EnqueueJob(ctx, store.EnqueueJobParams{Kind: kind, Payload: b, RunAt: at, DedupeKey: &key, MaxAttempts: attempts})
}

// enqueueQuery schedules status query n of a chain. tag keeps chains apart
// (the safety net after acceptance, or one started by an unclear answer).
func enqueueQuery(ctx context.Context, q *store.Queries, txID uuid.UUID, tag string, n int, at time.Time) error {
	if n > maxStatusQueries {
		return nil
	}
	return enqueue(ctx, q, JobStatusQuery, fmt.Sprintf("%s:%s:%s:%d", JobStatusQuery, txID, tag, n),
		map[string]any{"transaction_id": txID, "tag": tag, "n": n}, at, 5)
}

// Disburse sends an approved batch through M-Pesa B2C instead of the file.
func (s *Service) Disburse(ctx context.Context, p *auth.Principal, id uuid.UUID, m auth.Meta) (Batch, error) {
	if s.b2c == nil {
		return Batch{}, ErrB2COff
	}
	env := s.b2c.Environment
	return s.transition(ctx, p, id, m, "payout_batch.disbursed", "", func(q *store.Queries, b store.LockPayoutBatchRow) (int64, error) {
		if b.Status != "approved" {
			return 0, ErrWrongState
		}
		if err := covered(ctx, q, id); err != nil {
			return 0, err
		}
		n, err := q.DisbursePayoutBatch(ctx, store.DisbursePayoutBatchParams{DisbursedBy: &p.UserID, ID: id})
		if err != nil || n == 0 {
			return n, err
		}
		lines, err := q.PendingPayoutLines(ctx, id)
		if err != nil {
			return 0, err
		}
		now := s.now()
		for _, l := range lines {
			txID, err := q.InsertMpesaTransaction(ctx, store.InsertMpesaTransactionParams{PayoutLineID: l.ID,
				OriginatorConversationID: OriginatorID(l.ID), Environment: env, AmountCents: l.AmountCents,
				Currency: l.Currency, CreatedBy: p.UserID})
			if err != nil {
				return 0, err
			}
			if err := enqueue(ctx, q, JobB2CSend, JobB2CSend+":"+txID.String(), map[string]any{"transaction_id": txID}, now, 6); err != nil {
				return 0, err
			}
		}
		return n, nil
	})
}

// StopUnsent fails a line whose payment was never accepted by M-Pesa (still
// queued, for example after the gateway kept refusing). Nothing was paid; the
// amount stays owed.
func (s *Service) StopUnsent(ctx context.Context, p *auth.Principal, lineID uuid.UUID, reason string, m auth.Meta) (Batch, error) {
	r, err := note("reason", reason, true)
	if err != nil {
		return Batch{}, err
	}
	var batchID uuid.UUID
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		txID, err := q.MpesaTransactionIDByLine(ctx, lineID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		t, err := q.LockMpesaTransaction(ctx, txID)
		if err != nil {
			return err
		}
		batchID = t.BatchID
		if t.Status != "queued" {
			return ErrWrongState
		}
		return s.fail(ctx, q, t, &p.UserID, m, "stopped", "Stopped before M-Pesa accepted it: "+r)
	})
	if err != nil {
		return Batch{}, err
	}
	return s.Get(ctx, p, batchID)
}

// Handlers are the B2C job handlers for the jobs runner.
func (s *Service) Handlers() map[string]jobs.Handler {
	return map[string]jobs.Handler{
		JobB2CSend: func(ctx context.Context, payload json.RawMessage) error {
			var p struct {
				TransactionID uuid.UUID `json:"transaction_id"`
			}
			if err := json.Unmarshal(payload, &p); err != nil {
				return err
			}
			return s.SendPayment(ctx, p.TransactionID)
		},
		JobStatusQuery: func(ctx context.Context, payload json.RawMessage) error {
			var p struct {
				TransactionID uuid.UUID `json:"transaction_id"`
				Tag           string    `json:"tag"`
				N             int       `json:"n"`
			}
			if err := json.Unmarshal(payload, &p); err != nil {
				return err
			}
			return s.QueryStatus(ctx, p.TransactionID, p.Tag, p.N)
		},
		JobCallback: func(ctx context.Context, payload json.RawMessage) error {
			var p struct {
				CallbackID uuid.UUID `json:"callback_id"`
			}
			if err := json.Unmarshal(payload, &p); err != nil {
				return err
			}
			return s.ProcessCallback(ctx, p.CallbackID)
		},
	}
}

// SendPayment sends one transaction's B2C request, at most once.
func (s *Service) SendPayment(ctx context.Context, txID uuid.UUID) error {
	if s.b2c == nil {
		return ErrB2COff
	}
	var t store.LockMpesaTransactionRow
	send := false
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if t, err = q.LockMpesaTransaction(ctx, txID); err != nil {
			return err
		}
		switch t.Status {
		case "queued":
			send = true
			return q.MarkMpesaSending(ctx, txID)
		case "sending":
			// A worker stopped between marking and recording the answer. The
			// request may have gone out: never send again, ask instead.
			if t.SentAt != nil && s.now().Sub(*t.SentAt) < 2*time.Minute {
				return errSending
			}
			if err := q.MarkMpesaUnknown(ctx, store.MarkMpesaUnknownParams{ID: txID}); err != nil {
				return err
			}
			return enqueueQuery(ctx, q, txID, "crash", 1, s.now())
		}
		return nil
	})
	if err != nil || !send {
		return err
	}

	ack, callErr := s.b2c.Client.PayB2C(ctx, mpesa.Payment{
		OriginatorConversationID: t.OriginatorConversationID,
		AmountKES:                t.AmountCents / 100,
		Phone:                    strings.TrimPrefix(t.MpesaPhone, "+"),
		Remarks:                  "MezAI contributor payout",
		Occasion:                 "Payout",
	})

	var retry error
	err = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		cur, err := q.LockMpesaTransaction(ctx, txID)
		if err != nil {
			return err
		}
		if cur.Status != "sending" {
			return nil // a callback got here first
		}
		var rej *mpesa.RejectedError
		switch {
		case callErr == nil:
			if err := q.MarkMpesaAccepted(ctx, store.MarkMpesaAcceptedParams{ID: txID, ConversationID: &ack.ConversationID,
				ResponseCode: &ack.ResponseCode, ResponseDescription: &ack.ResponseDescription}); err != nil {
				return err
			}
			// A safety net in case the result callback never comes.
			return enqueueQuery(ctx, q, txID, "accepted", 1, s.now().Add(15*time.Minute))
		case errors.Is(callErr, mpesa.ErrNotAccepted):
			code, desc := "not_accepted", callErr.Error()
			retry = callErr
			return q.MarkMpesaRequeued(ctx, store.MarkMpesaRequeuedParams{ID: txID, ResponseCode: &code, ResponseDescription: &desc})
		case errors.As(callErr, &rej):
			// Keep Daraja's answer on the transaction, then fail it: nothing was paid.
			if err := q.MarkMpesaRequeued(ctx, store.MarkMpesaRequeuedParams{ID: txID, ResponseCode: &rej.Code, ResponseDescription: &rej.Description}); err != nil {
				return err
			}
			return s.fail(ctx, q, cur, nil, auth.Meta{}, rej.Code, "M-Pesa refused the request: "+describe(rej.Description, rej.Code))
		default:
			code, desc := "unknown", "No clear answer from M-Pesa; checking the status"
			if err := q.MarkMpesaUnknown(ctx, store.MarkMpesaUnknownParams{ID: txID, ResponseCode: &code, ResponseDescription: &desc}); err != nil {
				return err
			}
			s.log.Warn("mpesa b2c outcome unknown", slog.String("transaction_id", txID.String()), slog.Any("error", callErr))
			return enqueueQuery(ctx, q, txID, "unknown", 1, s.now().Add(statusQueryDelay(1)))
		}
	})
	if err != nil {
		return err
	}
	return retry
}

func describe(desc, code string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		desc = "no description"
	}
	if code != "" {
		desc += " (code " + code + ")"
	}
	if len(desc) > 480 {
		desc = desc[:480]
	}
	return desc
}

// QueryStatus sends status query n of a chain for an open transaction and
// schedules the next one as a safety net. Each query gets a fresh
// OriginatorConversationID ("-q" and the transaction's query count), so a
// retried job never reuses one. Asking again is harmless.
func (s *Service) QueryStatus(ctx context.Context, txID uuid.UUID, tag string, n int) error {
	if s.b2c == nil {
		return ErrB2COff
	}
	var t store.LockMpesaTransactionRow
	var k int32
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		var err error
		if t, err = q.LockMpesaTransaction(ctx, txID); err != nil {
			return err
		}
		if t.Status != "accepted" && t.Status != "unknown" && t.Status != "sending" {
			return nil
		}
		if k, err = q.CountMpesaStatusQuery(ctx, txID); err != nil {
			return err
		}
		return enqueueQuery(ctx, q, txID, tag, n+1, s.now().Add(statusQueryDelay(n+1)))
	})
	if err != nil || k == 0 {
		return err
	}
	ref := ""
	if t.Receipt != nil {
		ref = *t.Receipt
	} else if t.ConversationID != nil {
		ref = *t.ConversationID
	}
	_, err = s.b2c.Client.QueryStatus(ctx, mpesa.Query{OriginatorConversationID: queryOriginatorID(t.OriginatorConversationID, int(k)),
		PaymentOriginatorID: t.OriginatorConversationID, ReceiptOrConversation: ref})
	return err
}

// Callback is one stored callback, for the handler's answer.
type Callback struct {
	ID      uuid.UUID
	Verdict string
}

// MaxCallbackBytes bounds a callback body.
const MaxCallbackBytes = 64 << 10

// ReceiveCallback stores a callback with its verdict, queues its processing
// and then tries to process it at once. It errors only when storing failed,
// so Safaricom retries; a processing error is left to the queued job.
func (s *Service) ReceiveCallback(ctx context.Context, kind string, body []byte, ip *netip.Addr) (Callback, error) {
	verdict, note := "unmatched", ""
	var txID *uuid.UUID
	var cbID uuid.UUID
	var ocid, conv, code *string
	r, perr := mpesa.ParseResult(body)
	if !json.Valid(body) {
		b, _ := json.Marshal(map[string]string{"unreadable": string(body)})
		body = b
	}
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if perr != nil {
			note = "no Result in the body"
		} else {
			ocid, conv, code = nilIfEmpty(r.OriginatorConversationID), nilIfEmpty(r.ConversationID), nilIfEmpty(r.ResultCode)
			payment := r.OriginatorConversationID
			if kind == CallbackStatusResult || kind == CallbackStatusTimeout {
				payment = paymentOriginator(payment)
			}
			t, err := q.MpesaTransactionByOriginator(ctx, payment)
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				note = "no transaction with this OriginatorConversationID"
			case err != nil:
				return err
			default:
				txID = &t.ID
				verdict = "matched"
				// A B2C callback must also carry the ConversationID Daraja gave us.
				if (kind == CallbackB2CResult || kind == CallbackB2CTimeout) && t.ConversationID != nil &&
					r.ConversationID != "" && r.ConversationID != *t.ConversationID {
					verdict, note = "mismatch", "ConversationID differs from the accepted request"
				}
			}
		}
		var err error
		cbID, err = q.InsertMpesaCallback(ctx, store.InsertMpesaCallbackParams{Kind: kind, OriginatorConversationID: ocid,
			ConversationID: conv, TransactionID: txID, ResultCode: code, Body: body, RemoteIp: ip, Verdict: verdict, Note: nilIfEmpty(note)})
		if err != nil {
			return err
		}
		if verdict != "matched" {
			return nil
		}
		// Queued in the same commit: if the inline attempt below fails, the job retries.
		return enqueue(ctx, q, JobCallback, JobCallback+":"+cbID.String(), map[string]any{"callback_id": cbID},
			s.now().Add(time.Minute), 8)
	})
	if err != nil {
		return Callback{}, err
	}
	out := Callback{ID: cbID, Verdict: verdict}
	if verdict == "matched" {
		if err := s.ProcessCallback(ctx, out.ID); err != nil {
			s.log.Error("mpesa callback processing failed; the queued job will retry",
				slog.String("callback_id", out.ID.String()), slog.Any("error", err))
		}
	} else {
		s.log.Warn("mpesa callback not matched", slog.String("callback_id", out.ID.String()), slog.String("kind", kind),
			slog.String("verdict", verdict))
	}
	return out, nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// amountMatches compares a callback amount ("1200", "1200.00") with cents.
func amountMatches(v string, cents int64) bool {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return err == nil && int64(f*100+0.5) == cents
}

// ProcessCallback applies a stored, matched callback. It is idempotent.
func (s *Service) ProcessCallback(ctx context.Context, id uuid.UUID) error {
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		c, err := q.LockMpesaCallback(ctx, id)
		if err != nil {
			return err
		}
		if c.ProcessedAt != nil || c.Verdict != "matched" || c.TransactionID == nil {
			return nil
		}
		t, err := q.LockMpesaTransaction(ctx, *c.TransactionID)
		if err != nil {
			return err
		}
		r, err := mpesa.ParseResult(c.Body)
		if err != nil {
			return err
		}
		flag := func(note string) error {
			s.log.Warn("mpesa callback flagged", slog.String("callback_id", id.String()), slog.String("note", note))
			return q.FlagMpesaCallback(ctx, store.FlagMpesaCallbackParams{ID: id, Note: &note})
		}
		final := t.Status == "succeeded" || t.Status == "failed"

		switch c.Kind {
		case CallbackB2CResult:
			if r.ResultCode == "0" {
				receipt := strings.ToUpper(r.TransactionID)
				if receipt == "" {
					receipt = strings.ToUpper(r.Params["TransactionReceipt"])
				}
				if amt, ok := r.Params["TransactionAmount"]; !ok || !amountMatches(amt, t.AmountCents) {
					return flag("amount differs from the payout line")
				}
				return s.succeed(ctx, q, t, id, receipt, r, flag)
			}
			switch {
			case t.Status == "succeeded":
				return flag("conflicting result: failure reported after success")
			case final:
				return q.FinishMpesaCallback(ctx, id)
			}
			if err := s.fail(ctx, q, t, nil, auth.Meta{}, r.ResultCode, "M-Pesa could not pay: "+describe(r.ResultDesc, r.ResultCode)); err != nil {
				return err
			}

		case CallbackB2CTimeout:
			if !final {
				if err := q.MarkMpesaUnknown(ctx, store.MarkMpesaUnknownParams{ID: t.ID}); err != nil {
					return err
				}
				if err := enqueueQuery(ctx, q, t.ID, "timeout", 1, s.now()); err != nil {
					return err
				}
			}

		case CallbackStatusResult:
			if r.ResultCode == "0" {
				switch st := r.Params["TransactionStatus"]; st {
				case "Completed":
					if amt, ok := r.Params["Amount"]; ok && !amountMatches(amt, t.AmountCents) {
						return flag("amount differs from the payout line")
					}
					return s.succeed(ctx, q, t, id, strings.ToUpper(r.Params["ReceiptNo"]), r, flag)
				case "Failed", "Declined", "Cancelled", "Expired":
					switch {
					case t.Status == "succeeded":
						return flag("conflicting result: status " + st + " after success")
					case !final:
						if err := s.fail(ctx, q, t, nil, auth.Meta{}, "status_"+strings.ToLower(st),
							"M-Pesa reports the payment as "+strings.ToLower(st)); err != nil {
							return err
						}
					}
				}
				// Any other status (pending): the next query is already scheduled.
				break
			}
			// Not found (or the query failed). Give up only after three
			// answers and an hour since sending: never pay twice.
			if final {
				break
			}
			n, err := q.CountMpesaNotFound(ctx, t.ID)
			if err != nil {
				return err
			}
			if n >= 3 && t.SentAt != nil && s.now().Sub(*t.SentAt) >= time.Hour {
				if err := s.fail(ctx, q, t, nil, auth.Meta{}, "not_found",
					"M-Pesa has no record of this payment after "+strconv.Itoa(int(n))+" checks: "+describe(r.ResultDesc, r.ResultCode)); err != nil {
					return err
				}
			}

		case CallbackStatusTimeout:
			// The next status query is already scheduled.
		}
		return q.FinishMpesaCallback(ctx, id)
	})
	if err != nil {
		msg := err.Error()
		_ = s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
			return store.New(tx).FailMpesaCallback(ctx, store.FailMpesaCallbackParams{ID: id, ProcessError: &msg})
		})
	}
	return err
}

// succeed records a paid line and its payout ledger entry, once.
func (s *Service) succeed(ctx context.Context, q *store.Queries, t store.LockMpesaTransactionRow, callbackID uuid.UUID,
	receipt string, r mpesa.Result, flag func(string) error) error {
	if !receiptRe.MatchString(receipt) {
		return flag("success without a readable receipt")
	}
	switch t.Status {
	case "succeeded":
		if t.Receipt != nil && *t.Receipt == receipt {
			return q.FinishMpesaCallback(ctx, callbackID) // a duplicate
		}
		return flag("conflicting result: a second receipt " + receipt)
	case "failed":
		// Paid after we gave up on it: money left without a payout entry.
		// Ops must reconcile it by hand (an adjustment), never automatically.
		return flag("late_success: M-Pesa paid " + receipt + " after the line was recorded failed")
	}
	code := r.ResultCode
	desc := describe(r.ResultDesc, "")
	if err := q.MarkMpesaSucceeded(ctx, store.MarkMpesaSucceededParams{ID: t.ID, Receipt: &receipt, ResultCode: &code, ResultDesc: &desc}); err != nil {
		return mapPg(err)
	}
	n, err := q.RecordPayoutLinePaidB2C(ctx, store.RecordPayoutLinePaidB2CParams{ID: t.PayoutLineID, Receipt: &receipt})
	if err != nil {
		return mapPg(err)
	}
	if n == 0 {
		return fmt.Errorf("payouts: line %s already has a result", t.PayoutLineID)
	}
	if err := q.InsertPayoutLedgerEntry(ctx, store.InsertPayoutLedgerEntryParams{ContributorID: t.ContributorID,
		AmountCents: t.AmountCents, Currency: t.Currency, PayoutLineID: &t.PayoutLineID, CreatedBy: &t.CreatedBy}); err != nil {
		return err
	}
	lineID := t.PayoutLineID
	if err := auth.Audit(ctx, q, auth.Meta{}, auth.Entry{Action: "payout_line.paid", EntityType: "payout_line", EntityID: &lineID,
		After: map[string]any{"status": "paid", "amount_cents": t.AmountCents, "receipt": receipt, "via": "mpesa_b2c"}}); err != nil {
		return err
	}
	if err := s.settle(ctx, q, t.BatchID); err != nil {
		return err
	}
	return q.FinishMpesaCallback(ctx, callbackID)
}

// fail records a failed transaction and line. Nothing moves in the ledger:
// the amount stays owed and goes in the next batch.
func (s *Service) fail(ctx context.Context, q *store.Queries, t store.LockMpesaTransactionRow, actor *uuid.UUID, m auth.Meta,
	code, reason string) error {
	if err := q.MarkMpesaFailed(ctx, store.MarkMpesaFailedParams{ID: t.ID, ResultCode: &code, ResultDesc: &reason}); err != nil {
		return err
	}
	n, err := q.RecordPayoutLineFailedB2C(ctx, store.RecordPayoutLineFailedB2CParams{ID: t.PayoutLineID, Reason: &reason})
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("payouts: line %s already has a result", t.PayoutLineID)
	}
	lineID := t.PayoutLineID
	if err := auth.Audit(ctx, q, m, auth.Entry{Actor: actor, Action: "payout_line.failed", EntityType: "payout_line", EntityID: &lineID,
		Reason: reason, After: map[string]any{"status": "failed", "amount_cents": t.AmountCents, "via": "mpesa_b2c", "code": code}}); err != nil {
		return err
	}
	return s.settle(ctx, q, t.BatchID)
}

func (s *Service) settle(ctx context.Context, q *store.Queries, batchID uuid.UUID) error {
	n, err := q.SettleDisbursedBatch(ctx, batchID)
	if err != nil || n == 0 {
		return err
	}
	return auth.Audit(ctx, q, auth.Meta{}, auth.Entry{Action: "payout_batch.settled", EntityType: "payout_batch", EntityID: &batchID})
}

// SweepCallbacks queues processing for matched callbacks left unprocessed
// (the inline attempt and its job both failed, or the job ran out of tries).
func (s *Service) SweepCallbacks(ctx context.Context) (int, error) {
	var ids []uuid.UUID
	err := s.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		var err error
		ids, err = store.New(tx).UnprocessedMpesaCallbacks(ctx)
		return err
	})
	if err != nil {
		return 0, err
	}
	done := 0
	for _, id := range ids {
		if s.ProcessCallback(ctx, id) == nil {
			done++
		}
	}
	return done, nil
}

// RunCallbackSweep runs SweepCallbacks every interval until ctx ends.
func (s *Service) RunCallbackSweep(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.SweepCallbacks(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("mpesa callback sweep", slog.Any("error", err))
			} else if n > 0 {
				s.log.Info("mpesa callbacks processed by the sweep", slog.Int("count", n))
			}
		}
	}
}

// Transaction is one B2C payment as Ops sees it (no phone, no name).
type Transaction struct {
	ID                  uuid.UUID  `json:"id"`
	LineID              uuid.UUID  `json:"line_id"`
	PublicCode          string     `json:"public_code"`
	Status              string     `json:"status"`
	AmountCents         int64      `json:"amount_cents"`
	Currency            string     `json:"currency"`
	Environment         string     `json:"environment"`
	OriginatorID        string     `json:"originator_conversation_id"`
	SentAt              *time.Time `json:"sent_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	ResponseCode        *string    `json:"response_code"`
	ResponseDescription *string    `json:"response_description"`
	ResultCode          *string    `json:"result_code"`
	ResultDesc          *string    `json:"result_desc"`
	Receipt             *string    `json:"receipt"`
	StatusQueries       int32      `json:"status_queries"`
}

// Transactions lists a batch's B2C payments.
func (s *Service) Transactions(ctx context.Context, p *auth.Principal, batchID uuid.UUID) ([]Transaction, error) {
	out := []Transaction{}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if _, err := q.GetPayoutBatch(ctx, batchID); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		rows, err := q.ListBatchMpesaTransactions(ctx, batchID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			out = append(out, Transaction{ID: r.ID, LineID: r.PayoutLineID, PublicCode: r.PublicCode, Status: r.Status,
				AmountCents: r.AmountCents, Currency: r.Currency, Environment: r.Environment, OriginatorID: r.OriginatorConversationID,
				SentAt: r.SentAt, CompletedAt: r.CompletedAt, ResponseCode: r.ResponseCode, ResponseDescription: r.ResponseDescription,
				ResultCode: r.ResultCode, ResultDesc: r.ResultDesc, Receipt: r.Receipt, StatusQueries: r.StatusQueries})
		}
		return nil
	})
	return out, err
}

// ReconBatch is one disbursed batch: lines, transactions and ledger agree
// when Balanced.
type ReconBatch struct {
	ID               uuid.UUID  `json:"id"`
	PeriodStart      string     `json:"period_start"`
	PeriodEnd        string     `json:"period_end"`
	Status           string     `json:"status"`
	TotalCents       int64      `json:"total_cents"`
	LineCount        int32      `json:"line_count"`
	PaidLines        int32      `json:"paid_lines"`
	FailedLines      int32      `json:"failed_lines"`
	PendingLines     int32      `json:"pending_lines"`
	PaidCents        int64      `json:"paid_cents"`
	SucceededCents   int64      `json:"succeeded_cents"`
	LedgerCents      int64      `json:"ledger_cents"`
	OpenTransactions int32      `json:"open_transactions"`
	DisbursedAt      *time.Time `json:"disbursed_at"`
	SettledAt        *time.Time `json:"settled_at"`
	Balanced         bool       `json:"balanced"`
}

// Exception is something Ops must look at.
type Exception struct {
	Kind          string     `json:"kind"` // stuck, late_success, callback
	TransactionID *uuid.UUID `json:"transaction_id"`
	CallbackID    *uuid.UUID `json:"callback_id"`
	BatchID       *uuid.UUID `json:"batch_id"`
	PublicCode    *string    `json:"public_code"`
	Status        string     `json:"status"`
	AmountCents   *int64     `json:"amount_cents"`
	Note          string     `json:"note"`
	At            time.Time  `json:"at"`
}

// Reconciliation is the B2C reconciliation report.
type Reconciliation struct {
	B2C        B2CStatus    `json:"b2c"`
	Batches    []ReconBatch `json:"batches"`
	Exceptions []Exception  `json:"exceptions"`
}

// Reconcile builds the report: per batch, paid lines = succeeded
// transactions = payout ledger entries; and every open question.
func (s *Service) Reconcile(ctx context.Context, p *auth.Principal) (Reconciliation, error) {
	out := Reconciliation{B2C: s.B2CInfo(), Batches: []ReconBatch{}, Exceptions: []Exception{}}
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		bs, err := q.MpesaBatchReconciliation(ctx)
		if err != nil {
			return err
		}
		for _, b := range bs {
			out.Batches = append(out.Batches, ReconBatch{ID: b.ID, PeriodStart: day(b.PeriodStart), PeriodEnd: day(b.PeriodEnd),
				Status: b.Status, TotalCents: b.TotalCents, LineCount: b.LineCount, PaidLines: b.PaidLines, FailedLines: b.FailedLines,
				PendingLines: b.PendingLines, PaidCents: b.PaidCents, SucceededCents: b.SucceededCents, LedgerCents: b.LedgerCents,
				OpenTransactions: b.OpenTransactions, DisbursedAt: b.DisbursedAt, SettledAt: b.SettledAt,
				Balanced: b.PaidCents == b.SucceededCents && b.PaidCents == b.LedgerCents})
		}
		stuck, err := q.MpesaStuckTransactions(ctx)
		if err != nil {
			return err
		}
		for _, t := range stuck {
			note := "No result after a day; status checks continue"
			if t.Status == "queued" {
				note = "Not accepted by M-Pesa for over an hour; it can be stopped"
			}
			out.Exceptions = append(out.Exceptions, Exception{Kind: "stuck", TransactionID: ptr(t.ID), BatchID: ptr(t.BatchID),
				PublicCode: ptr(t.PublicCode), Status: t.Status, AmountCents: ptr(t.AmountCents), Note: note, At: t.UpdatedAt})
		}
		late, err := q.MpesaLateResults(ctx)
		if err != nil {
			return err
		}
		lateIDs := map[uuid.UUID]bool{}
		for _, l := range late {
			lateIDs[l.CallbackID] = true
			out.Exceptions = append(out.Exceptions, Exception{Kind: "late_success", TransactionID: ptr(l.TransactionID),
				CallbackID: ptr(l.CallbackID), BatchID: ptr(l.BatchID), PublicCode: ptr(l.PublicCode), Status: "succeeded",
				AmountCents: ptr(l.AmountCents), Note: "M-Pesa paid after the line was recorded failed: record an adjustment", At: l.CreatedAt})
		}
		cbs, err := q.MpesaCallbackExceptions(ctx)
		if err != nil {
			return err
		}
		for _, c := range cbs {
			if lateIDs[c.ID] {
				continue
			}
			note := c.Verdict
			if c.Note != nil {
				note = *c.Note
			}
			out.Exceptions = append(out.Exceptions, Exception{Kind: "callback", TransactionID: c.TransactionID,
				CallbackID: ptr(c.ID), Status: c.Verdict, Note: note, At: c.CreatedAt})
		}
		return nil
	})
	return out, err
}
