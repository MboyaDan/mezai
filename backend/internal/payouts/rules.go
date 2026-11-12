// Package payouts runs payout batches on the manual M-Pesa path (Stage 14a,
// brief 9 and 13, AC-11): Ops creates a batch for a period, a second person
// approves it, the system exports the bulk payment file, Ops pays in the
// M-Pesa portal and records each line as paid (with the receipt) or failed
// (with a reason the contributor sees).
//
// Money leaves the contributor's ledger balance only when a line is
// recorded paid (D-127). Only one batch is open at a time, so no earning is
// batched twice.
package payouts

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Hold reasons: why someone owed money is not in the batch.
const (
	HoldNoPhone            = "no_phone"
	HoldPhoneUnconfirmed   = "phone_unconfirmed"
	HoldIdentityUnverified = "identity_unverified"
	HoldNameUnchecked      = "name_unchecked"
	HoldBelowMinimum       = "below_minimum"
)

// Candidate is a contributor owed money, as the eligibility rule sees them.
type Candidate struct {
	BalanceCents     int64 // ledger balance now
	CutoffCents      int64 // ledger balance at the end of the period
	Phone            *string
	PhoneConfirmed   bool
	IdentityVerified bool
	NameChecked      bool
}

// Decide returns the amount to pay (whole shillings) or why it is held
// (D-128). The amount is the smaller of the balance now and at the cutoff,
// so a later correction is respected and later earnings wait.
func Decide(c Candidate, minCents int64) (amountCents int64, hold string) {
	owed := min(c.BalanceCents, c.CutoffCents)
	amount := owed - owed%100
	switch {
	case c.Phone == nil || *c.Phone == "":
		return 0, HoldNoPhone
	case !c.PhoneConfirmed:
		return 0, HoldPhoneUnconfirmed
	case !c.IdentityVerified:
		return 0, HoldIdentityUnverified
	case !c.NameChecked:
		return 0, HoldNameUnchecked
	case amount <= 0 || amount < minCents:
		return 0, HoldBelowMinimum
	}
	return amount, ""
}

// Fields a bulk payment column can hold.
var fields = map[string]bool{
	"phone_254": true, "phone_e164": true, "amount_kes": true, "name": true, "reference": true, "public_code": true,
}

// Column is one column of the bulk payment file.
type Column struct {
	Header string `json:"header"`
	Field  string `json:"field"`
}

// Format is the bulk payment file layout (setting payout.export_format).
// Confirmed stays false until DG-03 confirms Safaricom's template.
type Format struct {
	Confirmed bool     `json:"confirmed"`
	Columns   []Column `json:"columns"`
}

// ParseFormat reads and checks the setting.
func ParseFormat(raw []byte) (Format, error) {
	var f Format
	if err := json.Unmarshal(raw, &f); err != nil {
		return Format{}, fmt.Errorf("payout.export_format: %w", err)
	}
	if len(f.Columns) == 0 {
		return Format{}, errors.New("payout.export_format: no columns")
	}
	for _, c := range f.Columns {
		if !fields[c.Field] || strings.TrimSpace(c.Header) == "" {
			return Format{}, fmt.Errorf("payout.export_format: bad column %+v", c)
		}
	}
	return f, nil
}

// Payee is one line of the file.
type Payee struct {
	PhoneE164   string // +2547XXXXXXXX
	AmountCents int64  // whole shillings
	Name        string
	PublicCode  string
}

// Reference is the payment reference: the contributor's code and the
// period end, so a receipt can be matched back to the line.
func Reference(code string, periodEnd time.Time) string {
	return "MEZAI " + code + " " + periodEnd.Format(time.DateOnly)
}

// safeText stops a spreadsheet from reading a cell as a formula.
func safeText(s string) string {
	s = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 {
			return ' '
		}
		return r
	}, s))
	if s != "" && strings.ContainsRune("=+-@", rune(s[0])) {
		return "'" + s
	}
	return s
}

// BuildCSV writes the bulk payment file: a header row, then one row per
// payee, with CRLF line ends as spreadsheet tools expect.
func BuildCSV(f Format, payees []Payee, periodEnd time.Time) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	head := make([]string, len(f.Columns))
	for i, c := range f.Columns {
		head[i] = c.Header
	}
	if err := w.Write(head); err != nil {
		return nil, err
	}
	for _, p := range payees {
		if p.AmountCents <= 0 || p.AmountCents%100 != 0 {
			return nil, fmt.Errorf("payout amount %d is not whole shillings", p.AmountCents)
		}
		row := make([]string, len(f.Columns))
		for i, c := range f.Columns {
			switch c.Field {
			case "phone_254":
				row[i] = strings.TrimPrefix(p.PhoneE164, "+")
			case "phone_e164":
				row[i] = p.PhoneE164
			case "amount_kes":
				row[i] = strconv.FormatInt(p.AmountCents/100, 10)
			case "name":
				row[i] = safeText(p.Name)
			case "reference":
				row[i] = safeText(Reference(p.PublicCode, periodEnd))
			case "public_code":
				row[i] = p.PublicCode
			}
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
