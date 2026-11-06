package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/db"
	mailer "github.com/MboyaDan/mezai/backend/internal/mail"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// EnquiryDeps configures the public project enquiry form (brief 6.1, Contact).
type EnquiryDeps struct {
	// NotifyTo receives every enquiry by email.
	NotifyTo string
	// Turnstile checks the form's challenge token when set (DG-07). Nil
	// leaves it off; the honeypot and rate limits still apply.
	Turnstile Verifier
}

// Choice lists for the form. Keys are stored; labels go into the ops email.
// frontend/content/contact.ts carries the same keys, and a test compares them.
var (
	enquiryProjectTypes = map[string]string{
		"ai-evaluation":         "AI evaluation and RLHF",
		"african-language-data": "African language data",
		"data-annotation":       "Data annotation",
		"expert-review":         "Expert review",
		"not-sure":              "Not sure yet",
	}
	enquiryVolumes = map[string]string{
		"":          "Not given",
		"under-1k":  "Under 1,000 items",
		"1k-10k":    "1,000 to 10,000 items",
		"10k-100k":  "10,000 to 100,000 items",
		"over-100k": "More than 100,000 items",
		"not-sure":  "Not sure yet",
	}
	enquiryTimelines = map[string]string{
		"":          "Not given",
		"1-month":   "Within a month",
		"1-3-month": "In one to three months",
		"later":     "Later than three months",
		"exploring": "Just exploring",
	}
)

type enquiryRequest struct {
	Name         string `json:"name"`
	Organisation string `json:"organisation"`
	Email        string `json:"email"`
	ProjectType  string `json:"project_type"`
	Languages    string `json:"languages"`
	Volume       string `json:"volume"`
	Timeline     string `json:"timeline"`
	Message      string `json:"message"`
	// Website is the honeypot: hidden from people, filled in by bots.
	Website string `json:"website"`
	// TurnstileToken is the challenge answer when DG-07 is cleared.
	TurnstileToken string `json:"turnstile_token"`
}

// validate trims the request and returns per-field error codes.
func (r *enquiryRequest) validate() map[string]string {
	errs := map[string]string{}
	line := func(field string, v *string, required bool, max int) {
		*v = strings.TrimSpace(*v)
		switch {
		case *v == "" && required:
			errs[field] = "required"
		case utf8.RuneCountInString(*v) > max:
			errs[field] = "too_long"
		case strings.IndexFunc(*v, unicode.IsControl) >= 0:
			errs[field] = "invalid"
		}
	}
	line("name", &r.Name, true, 120)
	line("organisation", &r.Organisation, true, 200)
	line("languages", &r.Languages, false, 200)

	r.Email = normEmail(r.Email)
	if a, err := mail.ParseAddress(r.Email); r.Email == "" {
		errs["email"] = "required"
	} else if err != nil || a.Address != r.Email || len(r.Email) > 254 {
		errs["email"] = "invalid"
	}

	if _, ok := enquiryProjectTypes[r.ProjectType]; !ok {
		errs["project_type"] = "required"
	}
	if _, ok := enquiryVolumes[r.Volume]; !ok {
		errs["volume"] = "invalid"
	}
	if _, ok := enquiryTimelines[r.Timeline]; !ok {
		errs["timeline"] = "invalid"
	}

	// The message may span lines, but no other control characters.
	r.Message = strings.TrimSpace(strings.ReplaceAll(r.Message, "\r\n", "\n"))
	switch {
	case utf8.RuneCountInString(r.Message) > 5000:
		errs["message"] = "too_long"
	case strings.IndexFunc(r.Message, func(c rune) bool { return unicode.IsControl(c) && c != '\n' && c != '\t' }) >= 0:
		errs["message"] = "invalid"
	}
	return errs
}

type enquiryHandlers struct {
	pool     *db.Pool
	log      *slog.Logger
	deps     EnquiryDeps
	perEmail *limiter
}

// create saves an enquiry, queues the ops email and audits it, in one transaction.
func (h *enquiryHandlers) create(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	var req enquiryRequest
	if !bind(c, &req) {
		return
	}
	received := func() { c.JSON(http.StatusAccepted, gin.H{"status": "received"}) }
	if strings.TrimSpace(req.Website) != "" {
		// Looks like success so the bot learns nothing; nothing is stored.
		h.log.Info("enquiry honeypot", slog.String("request_id", RequestIDFrom(c)))
		received()
		return
	}
	if errs := req.validate(); len(errs) > 0 {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_enquiry", "fields": errs})
		return
	}
	if ok, wait := h.perEmail.allow(req.Email); !ok {
		tooMany(c, wait)
		return
	}
	if h.deps.Turnstile != nil {
		ok, err := h.deps.Turnstile.Verify(c.Request.Context(), req.TurnstileToken, c.ClientIP())
		if err != nil {
			h.log.Error("turnstile", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
			abort(c, http.StatusServiceUnavailable, "challenge_unavailable")
			return
		}
		if !ok {
			abort(c, http.StatusUnprocessableEntity, "challenge_failed")
			return
		}
	}

	ctx := c.Request.Context()
	err := h.pool.InScope(ctx, db.System(), func(tx pgx.Tx) error {
		q := store.New(tx)
		row, err := q.CreateEnquiry(ctx, store.CreateEnquiryParams{
			Name: req.Name, OrganisationName: req.Organisation, Email: req.Email, ProjectType: req.ProjectType,
			Languages: req.Languages, VolumeRange: req.Volume, Timeline: req.Timeline, Message: req.Message,
		})
		if err != nil {
			return err
		}
		orNone := func(s string) string {
			if s == "" {
				return "Not given"
			}
			return s
		}
		if err := mailer.Enqueue(ctx, q, h.deps.NotifyTo, mailer.TemplateEnquiry, map[string]string{
			"ID": row.ID.String(), "Name": req.Name, "Organisation": req.Organisation, "Email": req.Email,
			"ProjectType": enquiryProjectTypes[req.ProjectType], "Languages": orNone(req.Languages),
			"Volume": enquiryVolumes[req.Volume], "Timeline": enquiryTimelines[req.Timeline], "Message": orNone(req.Message),
		}); err != nil {
			return err
		}
		// Personal details stay in the enquiry row, not the audit log.
		return auth.Audit(ctx, q, meta(c), auth.Entry{
			Action: "enquiry.create", EntityType: "enquiry", EntityID: &row.ID,
			After: map[string]string{"project_type": req.ProjectType, "status": "new"},
		})
	})
	if err != nil {
		h.log.Error("enquiry", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		abort(c, http.StatusInternalServerError, "internal_error")
		return
	}
	received()
}

// Verifier checks a public form's challenge token (Cloudflare Turnstile, DG-07).
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

// Turnstile verifies tokens with Cloudflare's siteverify endpoint.
type Turnstile struct {
	Secret   string
	Endpoint string // defaults to Cloudflare's; tests point it at a fake
	Client   *http.Client
}

const turnstileEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

func (t Turnstile) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if strings.TrimSpace(token) == "" || len(token) > 2048 {
		return false, nil
	}
	endpoint, client := t.Endpoint, t.Client
	if endpoint == "" {
		endpoint = turnstileEndpoint
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	form := url.Values{"secret": {t.Secret}, "response": {token}}
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, &turnstileError{status: res.StatusCode}
	}
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 1<<16)).Decode(&out); err != nil {
		return false, err
	}
	return out.Success, nil
}

type turnstileError struct{ status int }

func (e *turnstileError) Error() string {
	return "turnstile: siteverify returned " + http.StatusText(e.status)
}
