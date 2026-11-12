// Package httpapi wires the Gin router, middleware and handlers.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/contributors"
	"github.com/MboyaDan/mezai/backend/internal/db"
	"github.com/MboyaDan/mezai/backend/internal/delivery"
	"github.com/MboyaDan/mezai/backend/internal/earnings"
	"github.com/MboyaDan/mezai/backend/internal/payouts"
	"github.com/MboyaDan/mezai/backend/internal/portal"
	"github.com/MboyaDan/mezai/backend/internal/projects"
	"github.com/MboyaDan/mezai/backend/internal/storage"
	"github.com/MboyaDan/mezai/backend/internal/tasks"
)

// Checker is a dependency that /readyz must reach, such as the database or object storage.
type Checker interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Log    *slog.Logger
	Checks map[string]Checker
	// Auth enables the /v1 API. Nil serves only the health endpoints (unit tests).
	Auth *AuthDeps
}

// AuthDeps configures sign-in and the /v1 API.
type AuthDeps struct {
	Service        *auth.Service
	Pool           *db.Pool // for public, read-only data such as stats
	AppOrigin      string
	CookieSecure   bool
	TrustedProxies []string
	Enquiry        EnquiryDeps
	// Files stores uploaded import files and request samples.
	Files storage.Blobs
	// IdentityKey keys National ID fingerprints (config.IdentityKey).
	IdentityKey []byte
}

// RouteRule is one /v1 route and the access rule it declared.
type RouteRule struct {
	Method, Path string
	Access       Access
}

// api registers /v1 routes. Every route must state its Access rule, so none
// can be added without an authorisation decision (AC-01).
type api struct {
	group *gin.RouterGroup
	rules []RouteRule
}

func (a *api) handle(method, path string, acc Access, h ...gin.HandlerFunc) {
	a.rules = append(a.rules, RouteRule{Method: method, Path: a.group.BasePath() + path, Access: acc})
	a.group.Handle(method, path, append([]gin.HandlerFunc{enforce(acc)}, h...)...)
}

func init() { gin.SetMode(gin.ReleaseMode) }

// NewRouter builds the API router.
func NewRouter(d Deps) *gin.Engine {
	r, _ := NewRouterWithRules(d)
	return r
}

// NewRouterWithRules also returns the access rule of every /v1 route, for tests.
func NewRouterWithRules(d Deps) (*gin.Engine, []RouteRule) {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(RequestID(), AccessLog(d.Log), Recover(d.Log))

	r.GET("/healthz", healthz)
	r.GET("/readyz", readyz(d.Log, d.Checks))

	var rules []RouteRule
	if d.Auth != nil {
		// Only these proxies' X-Forwarded-For is believed (rate limits, audit IPs).
		if err := r.SetTrustedProxies(d.Auth.TrustedProxies); err != nil {
			d.Log.Error("invalid TRUSTED_PROXIES", slog.Any("error", err))
		}
		v1 := r.Group("/v1", loadSession(d.Auth.Service, d.Log), csrf(d.Auth.AppOrigin))
		a := &api{group: v1}
		h := &authHandlers{svc: d.Auth.Service, log: d.Log, cookieSecure: d.Auth.CookieSecure, perEmail: newLimiter(10, 15*time.Minute)}
		perIP := rateLimit(newLimiter(20, time.Minute))
		a.handle(http.MethodPost, "/auth/login", Public(), perIP, h.login)
		a.handle(http.MethodPost, "/auth/logout", SignedIn(), h.logout)
		a.handle(http.MethodGet, "/auth/me", SignedIn(), h.me)

		// Public account links share one per-IP limit; email-keyed ones add a per-address limit.
		ah := &accountHandlers{svc: d.Auth.Service, log: d.Log, perEmail: newLimiter(5, time.Hour)}
		perIPLinks := rateLimit(newLimiter(20, time.Minute))
		a.handle(http.MethodPost, "/auth/password-reset/request", Public(), perIPLinks, ah.requestReset)
		a.handle(http.MethodPost, "/auth/password-reset/confirm", Public(), perIPLinks, ah.confirmReset)
		a.handle(http.MethodPost, "/auth/verify-email", Public(), perIPLinks, ah.verifyEmail)
		a.handle(http.MethodPost, "/auth/verify-email/resend", Public(), perIPLinks, ah.resendVerification)
		a.handle(http.MethodPost, "/invitations/accept", Public(), perIPLinks, ah.acceptInvitation)

		invite := Perm("user.manage", "org.user.invite")
		a.handle(http.MethodGet, "/invitations", invite, ah.listInvitations)
		a.handle(http.MethodPost, "/invitations", invite, ah.invite)
		a.handle(http.MethodDelete, "/invitations/:id", invite, ah.revokeInvitation)

		a.handle(http.MethodGet, "/users", Perm("user.manage"), ah.listUsers)
		a.handle(http.MethodPost, "/users/:id/roles", Perm("user.manage"), ah.grantRole)
		a.handle(http.MethodDelete, "/users/:id/roles/:role", Perm("user.manage"), ah.removeRole)
		a.handle(http.MethodPost, "/users/:id/status", Perm("user.manage"), ah.setStatus)

		a.handle(http.MethodGet, "/roles", Perm("user.manage", "role.manage"), ah.listRoles)
		a.handle(http.MethodGet, "/permissions", Perm("role.manage"), ah.listPermissions)
		a.handle(http.MethodPut, "/roles/:key/permissions", Perm("role.manage"), ah.setRolePermissions)

		a.handle(http.MethodGet, "/stats", Public(), listStats(d.Auth.Pool, d.Log))

		// Public project enquiries: honeypot, per-IP and per-address limits, Turnstile when configured.
		eh := &enquiryHandlers{pool: d.Auth.Pool, log: d.Log, deps: d.Auth.Enquiry, perEmail: newLimiter(3, 24*time.Hour)}
		a.handle(http.MethodPost, "/enquiries", Public(), rateLimit(newLimiter(5, time.Hour)), eh.create)

		// Contributor onboarding (Stage 7). Sign-up is public and answers the
		// same way for new and registered emails.
		ch := &contributorHandlers{auth: d.Auth.Service, svc: contributors.NewService(d.Auth.Pool, d.Auth.IdentityKey, d.Auth.Files), log: d.Log, perEmail: newLimiter(5, time.Hour)}
		a.handle(http.MethodPost, "/contributors/signup", Public(), rateLimit(newLimiter(10, time.Hour)), ch.signUp)
		a.handle(http.MethodGet, "/me/contributor", Perm("profile.manage"), ch.myProfile)
		// Rate limited: a number already on another account is refused, so
		// saves must not become a way to test many numbers (D-078).
		a.handle(http.MethodPut, "/me/contributor/profile", Perm("profile.manage"), rateLimitUser(newLimiter(30, time.Hour)), ch.updateMyProfile)
		a.handle(http.MethodPost, "/me/contributor/consents", Perm("profile.manage"), ch.acceptConsents)
		a.handle(http.MethodPut, "/me/contributor/cv", Perm("profile.manage"), rateLimitUser(newLimiter(10, time.Hour)), ch.uploadCV)
		a.handle(http.MethodDelete, "/me/contributor/cv", Perm("profile.manage"), ch.deleteCV)
		a.handle(http.MethodGet, "/me/qualification/tests", Perm("qualification.take"), ch.myTests)
		a.handle(http.MethodPost, "/me/qualification/tests/:id/attempts", Perm("qualification.take"), ch.startAttempt)
		a.handle(http.MethodGet, "/me/qualification/attempts/:id", Perm("qualification.take"), ch.myAttempt)
		a.handle(http.MethodPut, "/me/qualification/attempts/:id/answers", Perm("qualification.take"), ch.saveAnswers)
		a.handle(http.MethodPost, "/me/qualification/attempts/:id/submit", Perm("qualification.take"), ch.submitAttempt)

		// Ops: contributors, the qualification queue and tests.
		a.handle(http.MethodGet, "/contributors", Perm("contributor.approve", "contributor.pause"), ch.listContributors)
		a.handle(http.MethodGet, "/contributors/:id", Perm("contributor.approve", "contributor.pause"), ch.getContributor)
		a.handle(http.MethodPost, "/contributors/:id/status", Perm("contributor.pause"), ch.setContributorStatus)
		a.handle(http.MethodPost, "/contributors/:id/identity", Perm("contributor.approve"), ch.decideIdentity)
		a.handle(http.MethodPost, "/contributors/:id/mpesa-name-check", Perm("payout.create"), ch.recordMpesaNameCheck)
		a.handle(http.MethodGet, "/contributors/:id/cv", Perm("contributor.approve"), ch.downloadCV)
		a.handle(http.MethodPost, "/contributors/:id/screening", Perm("contributor.approve"), ch.decideScreening)
		a.handle(http.MethodPost, "/contributors/:id/interviews", Perm("contributor.approve"), ch.recordInterview)
		a.handle(http.MethodGet, "/qualification/attempts", Perm("contributor.approve"), ch.listAttempts)
		a.handle(http.MethodGet, "/qualification/attempts/:id", Perm("contributor.approve"), ch.reviewAttempt)
		a.handle(http.MethodPost, "/qualification/attempts/:id/decision", Perm("contributor.approve"), ch.decideAttempt)
		a.handle(http.MethodGet, "/qualification/tests", Perm("contributor.approve", "gold.manage"), ch.listTests)
		a.handle(http.MethodPost, "/qualification/tests", Perm("gold.manage"), ch.createTest)
		a.handle(http.MethodPost, "/qualification/core-check", Perm("gold.manage"), ch.createCoreCheck)
		a.handle(http.MethodPost, "/qualification/tests/:id/status", Perm("gold.manage"), ch.setTestStatus)

		// Task engine (Stage 9a). One task at a time; content only while reserved.
		th := &taskHandlers{svc: tasks.NewService(d.Auth.Pool, tasks.Options{NotifyTo: d.Auth.Enquiry.NotifyTo, AppOrigin: d.Auth.AppOrigin}), log: d.Log}
		a.handle(http.MethodGet, "/me/projects", Perm("task.submit"), th.myProjects)
		a.handle(http.MethodGet, "/me/projects/:id", Perm("task.submit"), th.myProject)
		// Per-person limits well above honest use (a task takes seconds at
		// least; autosave runs about every 1.5 s while typing), so one
		// account or script cannot hog the database (D-105).
		perMinute := func(n int) gin.HandlerFunc { return rateLimitUser(newLimiter(n, time.Minute)) }
		a.handle(http.MethodPost, "/me/projects/:id/next", Perm("task.submit"), perMinute(60), th.next)
		a.handle(http.MethodGet, "/me/tasks/:id", Perm("task.submit"), th.task)
		a.handle(http.MethodPut, "/me/tasks/:id/draft", Perm("task.submit"), perMinute(120), th.saveDraft)
		a.handle(http.MethodPost, "/me/tasks/:id/submit", Perm("task.submit"), perMinute(60), th.submit)
		a.handle(http.MethodPost, "/me/tasks/:id/skip", Perm("task.submit"), perMinute(60), th.skip)
		a.handle(http.MethodPost, "/me/tasks/:id/report", Perm("task.submit"), perMinute(30), th.report)
		a.handle(http.MethodGet, "/me/feedback", Perm("task.submit"), th.myFeedback)
		a.handle(http.MethodPost, "/me/feedback/:id/read", Perm("task.submit"), th.readFeedback)
		// Contributor dashboard (Stage 12a): own records only, readable while
		// paused (earnings.read_own, brief 4). Training lists eligible projects.
		eah := &earningsHandlers{svc: earnings.NewService(d.Auth.Pool), log: d.Log}
		a.handle(http.MethodGet, "/me/earnings", Perm("earnings.read_own"), eah.summary)
		a.handle(http.MethodGet, "/me/earnings/statement", Perm("earnings.read_own"), eah.statement)
		a.handle(http.MethodGet, "/me/payouts", Perm("earnings.read_own"), eah.payouts)
		a.handle(http.MethodGet, "/me/quality", Perm("earnings.read_own"), th.myQuality)
		a.handle(http.MethodGet, "/me/training", Perm("task.submit"), th.myTraining)
		// Quality picture: aggregates only, so a project's client may read it too.
		a.handle(http.MethodGet, "/projects/:id/quality", Perm("project.manage", "org.project.read"), th.projectQuality)
		a.handle(http.MethodPost, "/projects/:id/quality/snapshot", Perm("project.manage"), th.snapshotNow)
		a.handle(http.MethodGet, "/content-reports", Perm("project.manage"), th.listReports)
		a.handle(http.MethodPost, "/content-reports/:id/resolve", Perm("project.manage"), th.resolveReport)
		// Review and adjudication (Stage 11a). Reviewers see assigned projects;
		// review.adjudicate sees all (D-111). The service checks both.
		a.handle(http.MethodGet, "/review/items", Perm("review.read"), th.reviewQueue)
		a.handle(http.MethodGet, "/review/items/:id", Perm("review.read"), th.reviewItem)
		a.handle(http.MethodPost, "/review/items/:id/decision", Perm("review.decide"), perMinute(120), th.decide)
		a.handle(http.MethodGet, "/projects/:id/reviewers", Perm("project.manage"), th.projectReviewers)
		a.handle(http.MethodPost, "/projects/:id/reviewers", Perm("project.manage"), th.addReviewer)
		a.handle(http.MethodDelete, "/projects/:id/reviewers/:userId", Perm("project.manage"), th.removeReviewer)

		// Projects, imports, gold and client project requests (Stage 8).
		ph := &projectHandlers{log: d.Log, svc: projects.NewService(d.Auth.Pool, d.Auth.Files,
			projects.Options{NotifyTo: d.Auth.Enquiry.NotifyTo, AppOrigin: d.Auth.AppOrigin})}
		a.handle(http.MethodGet, "/organisations", Perm("org.manage", "project.manage"), ph.listOrganisations)
		a.handle(http.MethodPost, "/organisations", Perm("org.manage"), ph.createOrganisation)
		a.handle(http.MethodGet, "/task-templates", Perm("project.manage", "item.import", "gold.manage"), ph.taskTemplates)
		readProject := Perm("project.manage", "item.import", "gold.manage", "org.project.read")
		a.handle(http.MethodGet, "/projects", readProject, ph.listProjects)
		a.handle(http.MethodPost, "/projects", Perm("project.manage"), ph.createProject)
		a.handle(http.MethodGet, "/projects/:id", readProject, ph.getProject)
		a.handle(http.MethodPatch, "/projects/:id", Perm("project.manage"), ph.updateProject)
		a.handle(http.MethodPost, "/projects/:id/status", Perm("project.manage"), ph.setProjectStatus)
		// Item files need item.import and gold files gold.manage; the service checks the kind.
		a.handle(http.MethodGet, "/projects/:id/imports", Perm("item.import", "gold.manage", "org.project.read"), ph.listImports)
		a.handle(http.MethodPost, "/projects/:id/imports", Perm("item.import", "gold.manage"), rateLimit(newLimiter(60, time.Hour)), ph.checkImport)
		a.handle(http.MethodGet, "/imports/:id", Perm("item.import", "gold.manage", "org.project.read"), ph.getImport)
		a.handle(http.MethodPost, "/imports/:id/commit", Perm("item.import", "gold.manage"), ph.commitImport)
		a.handle(http.MethodGet, "/projects/:id/gold", Perm("gold.manage"), ph.listGold)
		a.handle(http.MethodPost, "/projects/:id/gold", Perm("gold.manage"), ph.createGold)
		a.handle(http.MethodPost, "/gold/:id/retire", Perm("gold.manage"), ph.retireGold)
		readRequest := Perm("project.manage", "org.project.request", "org.project.read")
		a.handle(http.MethodGet, "/project-requests", readRequest, ph.listRequests)
		a.handle(http.MethodPost, "/project-requests", Perm("org.project.request"), rateLimit(newLimiter(20, time.Hour)), ph.createRequest)
		a.handle(http.MethodGet, "/project-requests/:id", readRequest, ph.getRequest)
		a.handle(http.MethodGet, "/project-requests/:id/sample", readRequest, ph.requestSample)
		a.handle(http.MethodPost, "/project-requests/:id/convert", Perm("project.manage"), ph.convertRequest)
		a.handle(http.MethodPost, "/project-requests/:id/decline", Perm("project.manage"), ph.declineRequest)

		// Deliverables (Stage 11b). A client sees published versions of its
		// own projects only (RLS); a link works for one user for five minutes.
		dh := &deliveryHandlers{log: d.Log, svc: delivery.NewService(d.Auth.Pool, d.Auth.Files, d.Auth.IdentityKey, d.Log)}
		readDeliverables := Perm("deliverable.publish", "org.deliverable.download")
		a.handle(http.MethodGet, "/projects/:id/deliverables", readDeliverables, dh.list)
		a.handle(http.MethodPost, "/projects/:id/deliverables", Perm("deliverable.publish"), perMinute(10), dh.export)
		a.handle(http.MethodPost, "/projects/:id/deliverables/:version/publish", Perm("deliverable.publish"), dh.publish)
		a.handle(http.MethodPost, "/deliverables/:id/links", readDeliverables, perMinute(60), dh.link)
		a.handle(http.MethodGet, "/deliverables/:id/download", readDeliverables, perMinute(60), dh.download)

		// Client portal (Stage 13a). Clients read their own organisation
		// only; staff name the organisation (D-122 to D-124).
		pth := &portalHandlers{log: d.Log, svc: portal.NewService(d.Auth.Pool, d.Auth.Files, d.Auth.IdentityKey, d.Log)}
		a.handle(http.MethodGet, "/projects/:id/progress", Perm("project.manage", "org.project.read"), pth.progress)
		a.handle(http.MethodGet, "/projects/:id/workforce", Perm("project.manage", "org.project.read"), pth.workforce)
		a.handle(http.MethodGet, "/activity", Perm("audit.read", "org.project.read"), pth.activity)
		a.handle(http.MethodGet, "/team", Perm("user.manage", "org.user.invite"), pth.members)
		readInvoices := Perm("invoice.manage", "org.invoice.read")
		a.handle(http.MethodGet, "/invoices", readInvoices, pth.listInvoices)
		a.handle(http.MethodPost, "/invoices", Perm("invoice.manage"), rateLimit(newLimiter(60, time.Hour)), pth.uploadInvoice)
		a.handle(http.MethodPost, "/invoices/:id/status", Perm("invoice.manage"), pth.setInvoiceStatus)
		a.handle(http.MethodGet, "/invoices/:id/download", readInvoices, perMinute(60), pth.downloadInvoice)
		// Ops overview (Stage 13b).
		a.handle(http.MethodGet, "/ops/overview", Perm("project.manage"), perMinute(60), pth.opsOverview)

		// Payouts, manual M-Pesa path (Stage 14a, AC-11, D-127 and D-128).
		poh := &payoutHandlers{log: d.Log, svc: payouts.NewService(d.Auth.Pool, d.Auth.Files, d.Log)}
		readPayouts := Perm("payout.create", "payout.approve")
		a.handle(http.MethodGet, "/payouts/preview", Perm("payout.create"), poh.preview)
		a.handle(http.MethodGet, "/payouts/batches", readPayouts, poh.list)
		a.handle(http.MethodPost, "/payouts/batches", Perm("payout.create"), perMinute(10), poh.create)
		a.handle(http.MethodGet, "/payouts/batches/:id", readPayouts, poh.get)
		a.handle(http.MethodPost, "/payouts/batches/:id/submit", Perm("payout.create"), poh.action(poh.submit))
		a.handle(http.MethodPost, "/payouts/batches/:id/approve", Perm("payout.approve"), poh.action(poh.approve))
		a.handle(http.MethodPost, "/payouts/batches/:id/return", Perm("payout.approve"), poh.action(poh.sendBack))
		a.handle(http.MethodPost, "/payouts/batches/:id/cancel", Perm("payout.create"), poh.action(poh.cancel))
		a.handle(http.MethodPost, "/payouts/batches/:id/export", Perm("payout.create"), poh.action(poh.export))
		a.handle(http.MethodGet, "/payouts/batches/:id/file", readPayouts, perMinute(30), poh.download)
		a.handle(http.MethodPost, "/payouts/lines/:id/result", Perm("payout.create"), perMinute(240), poh.result)
		a.handle(http.MethodPut, "/payouts/next-date", Perm("payout.create"), poh.nextDate)
		rules = a.rules
	}

	r.NoRoute(func(c *gin.Context) { c.JSON(http.StatusNotFound, gin.H{"error": "not_found"}) })
	r.NoMethod(func(c *gin.Context) { c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method_not_allowed"}) })
	return r, rules
}

// healthz reports that the process is up. It never touches dependencies.
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// readyz checks every dependency with a short timeout. Failure details go to
// the log only; the response names the failing check without the error text.
func readyz(log *slog.Logger, checks map[string]Checker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		status, results := http.StatusOK, make(map[string]string, len(checks))
		for name, chk := range checks {
			if err := chk.Ping(ctx); err != nil {
				log.Warn("readiness check failed", slog.String("request_id", RequestIDFrom(c)), slog.String("check", name), slog.Any("error", err))
				results[name] = "unavailable"
				status = http.StatusServiceUnavailable
				continue
			}
			results[name] = "ok"
		}
		overall := "ok"
		if status != http.StatusOK {
			overall = "unavailable"
		}
		c.JSON(status, gin.H{"status": overall, "checks": results})
	}
}
