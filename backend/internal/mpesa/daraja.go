// Package mpesa talks to Safaricom's Daraja API for B2C payouts (Stage 15,
// DG-02): OAuth tokens, B2C payment requests, transaction status queries
// and the shape of their asynchronous callbacks. It knows nothing about the
// database; internal/payouts decides what to send and records the results.
//
// Errors are sorted by what they mean for money:
//   - ErrNotAccepted: the gateway refused before accepting (401, 429). The
//     same request may be sent again.
//   - *RejectedError: Daraja answered and refused the request. Nothing was
//     paid; nothing is sent again for that payout line.
//   - ErrUnknown: no clear answer (network error, timeout, 5xx, garbled
//     body). The request may have been accepted, so it is never sent again;
//     a status query settles it.
package mpesa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Base URLs of Daraja.
const (
	SandboxURL    = "https://sandbox.safaricom.co.ke"
	ProductionURL = "https://api.safaricom.co.ke"
)

var (
	ErrNotAccepted = errors.New("mpesa: request not accepted by the gateway; safe to send again")
	ErrUnknown     = errors.New("mpesa: outcome unknown; do not send again, query the status")
)

// RejectedError is a definite refusal: nothing was paid.
type RejectedError struct {
	Status      int
	Code        string
	Description string
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("mpesa: request refused (%d %s): %s", e.Status, e.Code, e.Description)
}

// Config is what a Client needs. SecurityCredential is the initiator
// password already encrypted with Safaricom's certificate (base64).
type Config struct {
	BaseURL            string
	ConsumerKey        string
	ConsumerSecret     string
	ShortCode          string
	InitiatorName      string
	SecurityCredential string
	// CallbackBase is the public URL prefix of our callback routes,
	// including the secret path token: .../v1/mpesa/callbacks/<token>.
	CallbackBase string
}

// Payment is one B2C request.
type Payment struct {
	OriginatorConversationID string
	AmountKES                int64  // whole shillings
	Phone                    string // 2547XXXXXXXX
	Remarks                  string
	Occasion                 string
}

// Query is one transaction status query for an earlier payment.
type Query struct {
	// OriginatorConversationID of this query (not of the payment).
	OriginatorConversationID string
	// PaymentOriginatorID is the payment's OriginatorConversationID.
	PaymentOriginatorID string
	// ReceiptOrConversation is the payment's M-Pesa receipt when known,
	// otherwise its ConversationID.
	ReceiptOrConversation string
}

// Ack is Daraja's synchronous answer to an accepted request.
type Ack struct {
	ConversationID           string
	OriginatorConversationID string
	ResponseCode             string
	ResponseDescription      string
}

// Client sends B2C requests and status queries.
type Client interface {
	PayB2C(ctx context.Context, p Payment) (Ack, error)
	QueryStatus(ctx context.Context, q Query) (Ack, error)
}

// Daraja is the HTTP client for Safaricom's API.
type Daraja struct {
	cfg  Config
	http *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
	now     func() time.Time
}

// NewDaraja returns a client. Requests time out after 30 seconds.
func NewDaraja(cfg Config, hc *http.Client) *Daraja {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.CallbackBase = strings.TrimRight(cfg.CallbackBase, "/")
	return &Daraja{cfg: cfg, http: hc, now: time.Now}
}

// Callback paths under CallbackBase.
const (
	PathB2CResult     = "/b2c/result"
	PathB2CTimeout    = "/b2c/timeout"
	PathStatusResult  = "/status/result"
	PathStatusTimeout = "/status/timeout"
)

// accessToken returns a cached OAuth token, refreshing it a minute early.
func (d *Daraja) accessToken(ctx context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.token != "" && d.now().Before(d.expires) {
		return d.token, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.cfg.BaseURL+"/oauth/v1/generate?grant_type=client_credentials", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(d.cfg.ConsumerKey, d.cfg.ConsumerSecret)
	res, err := d.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token request: %v", ErrNotAccepted, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: token request answered %d", ErrNotAccepted, res.StatusCode)
	}
	var tok struct {
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"` // Daraja sends a string
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("%w: token response unreadable", ErrNotAccepted)
	}
	secs := 3599
	_, _ = fmt.Sscan(strings.Trim(string(tok.ExpiresIn), `"`), &secs)
	d.token = tok.AccessToken
	d.expires = d.now().Add(time.Duration(secs)*time.Second - time.Minute)
	return d.token, nil
}

func (d *Daraja) dropToken() {
	d.mu.Lock()
	d.token = ""
	d.mu.Unlock()
}

// post sends one API request and sorts the answer (see the package comment).
// Getting a token cannot pay anyone, so its failures are ErrNotAccepted.
func (d *Daraja) post(ctx context.Context, path string, payload any) (Ack, error) {
	tok, err := d.accessToken(ctx)
	if err != nil {
		return Ack{}, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Ack{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return Ack{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := d.http.Do(req)
	if err != nil {
		// The request may have reached Safaricom.
		return Ack{}, fmt.Errorf("%w: %v", ErrUnknown, err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	return classify(res.StatusCode, body, d.dropToken)
}

// classify turns Daraja's synchronous answer into an Ack or one of the
// three error kinds.
func classify(status int, body []byte, dropToken func()) (Ack, error) {
	var r struct {
		ConversationID           string `json:"ConversationID"`
		OriginatorConversationID string `json:"OriginatorConversationID"`
		ResponseCode             string `json:"ResponseCode"`
		ResponseDescription      string `json:"ResponseDescription"`
		ErrorCode                string `json:"errorCode"`
		ErrorMessage             string `json:"errorMessage"`
	}
	parsed := json.Unmarshal(body, &r) == nil
	switch {
	case status == http.StatusOK && parsed && r.ResponseCode == "0" && r.ConversationID != "":
		return Ack{ConversationID: r.ConversationID, OriginatorConversationID: r.OriginatorConversationID,
			ResponseCode: r.ResponseCode, ResponseDescription: clip(r.ResponseDescription)}, nil
	case status == http.StatusOK && parsed && r.ResponseCode != "" && r.ResponseCode != "0":
		return Ack{}, &RejectedError{Status: status, Code: r.ResponseCode, Description: clip(r.ResponseDescription)}
	case status == http.StatusUnauthorized || status == http.StatusTooManyRequests:
		if status == http.StatusUnauthorized && dropToken != nil {
			dropToken()
		}
		return Ack{}, fmt.Errorf("%w: answered %d", ErrNotAccepted, status)
	case status >= 400 && status < 500 && parsed && (r.ErrorCode != "" || r.ErrorMessage != ""):
		return Ack{}, &RejectedError{Status: status, Code: r.ErrorCode, Description: clip(r.ErrorMessage)}
	default:
		// 5xx, a 200 we cannot read, or a 4xx without Daraja's error body
		// (a proxy, not Daraja): we cannot tell whether it was accepted.
		return Ack{}, fmt.Errorf("%w: answered %d", ErrUnknown, status)
	}
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		s = s[:400]
	}
	return s
}

// PayB2C sends one B2C payment request (API v3, which carries our
// OriginatorConversationID).
func (d *Daraja) PayB2C(ctx context.Context, p Payment) (Ack, error) {
	return d.post(ctx, "/mpesa/b2c/v3/paymentrequest", map[string]any{
		"OriginatorConversationID": p.OriginatorConversationID,
		"InitiatorName":            d.cfg.InitiatorName,
		"SecurityCredential":       d.cfg.SecurityCredential,
		"CommandID":                "BusinessPayment",
		"Amount":                   p.AmountKES,
		"PartyA":                   d.cfg.ShortCode,
		"PartyB":                   p.Phone,
		"Remarks":                  p.Remarks,
		"QueueTimeOutURL":          d.cfg.CallbackBase + PathB2CTimeout,
		"ResultURL":                d.cfg.CallbackBase + PathB2CResult,
		"Occassion":                p.Occasion, // Daraja's spelling
	})
}

// QueryStatus asks for the outcome of an earlier payment. The answer comes
// to the status callbacks.
func (d *Daraja) QueryStatus(ctx context.Context, q Query) (Ack, error) {
	return d.post(ctx, "/mpesa/transactionstatus/v1/query", map[string]any{
		"Initiator":                d.cfg.InitiatorName,
		"SecurityCredential":       d.cfg.SecurityCredential,
		"CommandID":                "TransactionStatusQuery",
		"TransactionID":            q.ReceiptOrConversation,
		"OriginalConversationID":   q.PaymentOriginatorID,
		"OriginatorConversationID": q.OriginatorConversationID,
		"PartyA":                   d.cfg.ShortCode,
		"IdentifierType":           "4", // organisation shortcode
		"ResultURL":                d.cfg.CallbackBase + PathStatusResult,
		"QueueTimeOutURL":          d.cfg.CallbackBase + PathStatusTimeout,
		"Remarks":                  "Payout status check",
		"Occasion":                 "",
	})
}

// Result is a parsed callback body ({"Result": {...}}).
type Result struct {
	ResultType               int
	ResultCode               string
	ResultDesc               string
	OriginatorConversationID string
	ConversationID           string
	TransactionID            string
	Params                   map[string]string
}

// ParseResult reads a result or timeout callback. ResultCode arrives as a
// number or a string; parameters' values as strings or numbers.
func ParseResult(body []byte) (Result, error) {
	var env struct {
		Result *struct {
			ResultType               json.Number     `json:"ResultType"`
			ResultCode               json.RawMessage `json:"ResultCode"`
			ResultDesc               string          `json:"ResultDesc"`
			OriginatorConversationID string          `json:"OriginatorConversationID"`
			ConversationID           string          `json:"ConversationID"`
			TransactionID            string          `json:"TransactionID"`
			ResultParameters         *struct {
				ResultParameter json.RawMessage `json:"ResultParameter"`
			} `json:"ResultParameters"`
		} `json:"Result"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&env); err != nil || env.Result == nil {
		return Result{}, errors.New("mpesa: callback has no Result")
	}
	r := env.Result
	out := Result{
		ResultCode:               strings.Trim(strings.TrimSpace(string(r.ResultCode)), `"`),
		ResultDesc:               clip(r.ResultDesc),
		OriginatorConversationID: strings.TrimSpace(r.OriginatorConversationID),
		ConversationID:           strings.TrimSpace(r.ConversationID),
		TransactionID:            strings.TrimSpace(r.TransactionID),
		Params:                   map[string]string{},
	}
	if n, err := r.ResultType.Int64(); err == nil {
		out.ResultType = int(n)
	}
	if r.ResultParameters != nil && len(r.ResultParameters.ResultParameter) > 0 {
		type kv struct {
			Key   string `json:"Key"`
			Value any    `json:"Value"`
		}
		var list []kv
		raw := r.ResultParameters.ResultParameter
		if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) { // a single parameter comes as an object
			raw = append(append([]byte("["), raw...), ']')
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(&list); err == nil {
			for _, p := range list {
				out.Params[p.Key] = strings.TrimSpace(fmt.Sprint(p.Value))
			}
		}
	}
	return out, nil
}
