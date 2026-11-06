package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/MboyaDan/mezai/backend/internal/imports"
	"github.com/MboyaDan/mezai/backend/internal/projects"
)

// projectHandlers serve organisations, projects, imports, gold and client
// project requests (Stage 8).
type projectHandlers struct {
	svc *projects.Service
	log *slog.Logger
}

func (h *projectHandlers) fail(c *gin.Context, err error) {
	var fe *projects.FieldError
	if errors.As(err, &fe) {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": fe.Field, "message": fe.Message})
		return
	}
	var nr *projects.NotReadyError
	if errors.As(err, &nr) {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"error": "not_ready", "missing": nr.Missing})
		return
	}
	status, code := http.StatusConflict, ""
	switch {
	case errors.Is(err, projects.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, projects.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, projects.ErrSlugTaken):
		code = "slug_taken"
	case errors.Is(err, projects.ErrIllegalTransition):
		code = "illegal_transition"
	case errors.Is(err, projects.ErrClosed):
		code = "project_closed"
	case errors.Is(err, projects.ErrImportNotOpen):
		code = "import_not_committable"
	case errors.Is(err, projects.ErrImportStale):
		code = "import_stale"
	case errors.Is(err, projects.ErrFileChanged):
		code = "file_changed"
	case errors.Is(err, projects.ErrAlreadyDecided):
		code = "already_decided"
	case errors.Is(err, projects.ErrStorage):
		h.log.Error("file storage failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusServiceUnavailable, "storage_unavailable"
	default:
		h.log.Error("project request failed", slog.String("request_id", RequestIDFrom(c)), slog.Any("error", err))
		status, code = http.StatusInternalServerError, "internal_error"
	}
	abort(c, status, code)
}

// maxProjectBody bounds JSON bodies (instructions are the largest field).
const maxProjectBody = 512 << 10

func (h *projectHandlers) bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProjectBody)
	return bind(c, v)
}

// upload reads one multipart file field, refusing anything over max bytes.
func upload(c *gin.Context, field string, max int64, required bool) (name string, data []byte, ok bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, max+64<<10)
	fh, err := c.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) && !required {
		return "", nil, true
	}
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large", "max_bytes": max})
			return "", nil, false
		}
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_input", "field": field, "message": "attach a file"})
		return "", nil, false
	}
	if fh.Size > max {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large", "max_bytes": max})
		return "", nil, false
	}
	f, err := fh.Open()
	if err != nil {
		abort(c, http.StatusBadRequest, "bad_request")
		return "", nil, false
	}
	defer f.Close()
	data, err = io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(data)) > max {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file_too_large", "max_bytes": max})
		return "", nil, false
	}
	return fh.Filename, data, true
}

func (h *projectHandlers) listOrganisations(c *gin.Context) {
	out, err := h.svc.ListOrganisations(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"organisations": out})
}

func (h *projectHandlers) createOrganisation(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.CreateOrganisation(c.Request.Context(), PrincipalFrom(c), req.Name, req.Slug, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) taskTemplates(c *gin.Context) {
	out, err := h.svc.TaskTemplates(c.Request.Context(), PrincipalFrom(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": out})
}

var projectStatuses = []string{"", projects.StatusDraft, projects.StatusReady, projects.StatusLive, projects.StatusPaused, projects.StatusClosed}

func (h *projectHandlers) listProjects(c *gin.Context) {
	status := c.Query("status")
	if !slices.Contains(projectStatuses, status) {
		abort(c, http.StatusBadRequest, "bad_request")
		return
	}
	var org *uuid.UUID
	if s := c.Query("org_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			abort(c, http.StatusBadRequest, "bad_request")
			return
		}
		org = &id
	}
	out, err := h.svc.ListProjects(c.Request.Context(), PrincipalFrom(c), org, status)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

func (h *projectHandlers) createProject(c *gin.Context) {
	var req projects.ProjectInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.CreateProject(c.Request.Context(), PrincipalFrom(c), req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) getProject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetProject(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *projectHandlers) updateProject(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req projects.ProjectInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.UpdateProject(c.Request.Context(), PrincipalFrom(c), id, req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *projectHandlers) setProjectStatus(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.SetStatus(c.Request.Context(), PrincipalFrom(c), id, req.Action, req.Reason, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *projectHandlers) listImports(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.ListImports(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"imports": out})
}

// checkImport is the dry run: multipart "file" plus "kind" (items or gold).
func (h *projectHandlers) checkImport(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	name, data, ok := upload(c, "file", imports.MaxBytes, true)
	if !ok {
		return
	}
	out, err := h.svc.CheckImport(c.Request.Context(), PrincipalFrom(c), id, c.PostForm("kind"), name, data, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) getImport(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetImport(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *projectHandlers) commitImport(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.CommitImport(c.Request.Context(), PrincipalFrom(c), id, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (h *projectHandlers) listGold(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.ListGold(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"gold": out})
}

func (h *projectHandlers) createGold(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req projects.GoldInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.CreateGold(c.Request.Context(), PrincipalFrom(c), id, req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) retireGold(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	if err := h.svc.RetireGold(c.Request.Context(), PrincipalFrom(c), id, meta(c)); err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "retired"})
}

var requestStatuses = []string{"", "submitted", "converted", "declined"}

func (h *projectHandlers) listRequests(c *gin.Context) {
	status := c.Query("status")
	if !slices.Contains(requestStatuses, status) {
		abort(c, http.StatusBadRequest, "bad_request")
		return
	}
	out, err := h.svc.ListRequests(c.Request.Context(), PrincipalFrom(c), status)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": out})
}

// createRequest takes JSON, or multipart with a "request" JSON field and an
// optional "sample" file.
func (h *projectHandlers) createRequest(c *gin.Context) {
	var req projects.RequestInput
	var sample *projects.Sample
	ct, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if ct == "multipart/form-data" {
		name, data, ok := upload(c, "sample", projects.MaxSampleBytes, false)
		if !ok {
			return
		}
		if err := jsonUnmarshalStrict([]byte(c.PostForm("request")), &req); err != nil {
			abort(c, http.StatusBadRequest, "bad_request")
			return
		}
		if data != nil {
			sample = &projects.Sample{Name: name, Data: data}
		}
	} else if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.CreateRequest(c.Request.Context(), PrincipalFrom(c), req, sample, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) getRequest(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	out, err := h.svc.GetRequest(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// requestSample downloads the attachment. It is always sent as an opaque
// download so a browser never renders it.
func (h *projectHandlers) requestSample(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	name, data, err := h.svc.RequestSample(c.Request.Context(), PrincipalFrom(c), id)
	if err != nil {
		h.fail(c, err)
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *projectHandlers) convertRequest(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req projects.ProjectInput
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.ConvertRequest(c.Request.Context(), PrincipalFrom(c), id, req, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (h *projectHandlers) declineRequest(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if !h.bind(c, &req) {
		return
	}
	out, err := h.svc.DeclineRequest(c.Request.Context(), PrincipalFrom(c), id, req.Note, meta(c))
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func jsonUnmarshalStrict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
