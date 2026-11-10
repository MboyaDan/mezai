package delivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/mezai/backend/internal/auth"
	"github.com/MboyaDan/mezai/backend/internal/store"
)

// LinkTTL is how long a download link works.
const LinkTTL = 5 * time.Minute

func deriveLinkKey(secret []byte) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte("mezai/deliverable-download-links/v1"))
	return m.Sum(nil)
}

// sign binds a link to one file, one user and an expiry time.
func (s *Service) sign(id uuid.UUID, file string, user uuid.UUID, expires int64) string {
	m := hmac.New(sha256.New, s.linkKey)
	fmt.Fprintf(m, "%s|%s|%s|%d", id, file, user, expires)
	return hex.EncodeToString(m.Sum(nil))
}

// Link is a short-lived download URL on the frontend origin (/api is
// rewritten to the API).
type Link struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func validFile(file string) bool { return file == "data" || file == "manifest" }

// CreateLink signs a download link for a file the caller can see: staff any
// version, a client only published versions of its own projects (RLS).
func (s *Service) CreateLink(ctx context.Context, p *auth.Principal, id uuid.UUID, file string) (Link, error) {
	if !validFile(file) {
		return Link{}, ErrInvalidInput
	}
	if _, err := s.visibleDeliverable(ctx, p, id); err != nil {
		return Link{}, err
	}
	exp := s.now().Add(LinkTTL).Truncate(time.Second)
	v := url.Values{"file": {file}, "expires": {strconv.FormatInt(exp.Unix(), 10)},
		"sig": {s.sign(id, file, p.UserID, exp.Unix())}}
	return Link{URL: fmt.Sprintf("/api/v1/deliverables/%s/download?%s", id, v.Encode()), ExpiresAt: exp}, nil
}

func (s *Service) visibleDeliverable(ctx context.Context, p *auth.Principal, id uuid.UUID) (store.GetDeliverableRow, error) {
	var d store.GetDeliverableRow
	err := s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		var err error
		d, err = store.New(tx).GetDeliverable(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return d, err
}

// Download is a file ready to send.
type Download struct {
	Name        string
	ContentType string
	Data        []byte
}

// Download checks a signed link, reads the file, verifies a data file
// against its checksum, and records the download (deliverable_downloads and
// the audit log) before returning it. The link only works for the user it
// was made for, and only while that user can still see the deliverable.
func (s *Service) Download(ctx context.Context, p *auth.Principal, id uuid.UUID, file, expires, sig string, m auth.Meta) (Download, error) {
	exp, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || !validFile(file) || len(sig) != 64 {
		return Download{}, ErrLinkInvalid
	}
	want := s.sign(id, file, p.UserID, exp)
	if !hmac.Equal([]byte(sig), []byte(want)) || s.now().Unix() > exp {
		return Download{}, ErrLinkInvalid
	}
	d, err := s.visibleDeliverable(ctx, p, id)
	if err != nil {
		return Download{}, err
	}
	key, name, ctype := d.FileKey, FileName(d.ProjectName, d.Version, d.Format), "text/csv; charset=utf-8"
	switch {
	case file == "manifest":
		key, name, ctype = d.ManifestKey, FileName(d.ProjectName, d.Version, "manifest"), "application/json"
	case d.Format == "jsonl":
		ctype = "application/x-ndjson"
	}
	data, err := s.blobs.Get(ctx, key, maxFileBytes)
	if err != nil {
		s.log.ErrorContext(ctx, "deliverable read failed", "err", err)
		return Download{}, ErrStorage
	}
	if file == "data" && checksum(data) != d.ChecksumSha256 {
		s.log.ErrorContext(ctx, "deliverable checksum mismatch", "deliverable", d.ID)
		return Download{}, ErrFileChanged
	}
	err = s.pool.InScope(ctx, p.Scope(), func(tx pgx.Tx) error {
		q := store.New(tx)
		if err := q.RecordDeliverableDownload(ctx, store.RecordDeliverableDownloadParams{OrgID: d.OrgID, DeliverableID: d.ID,
			UserID: p.UserID, Ip: m.IP, File: file}); err != nil {
			return err
		}
		return auth.Audit(ctx, q, m, auth.Entry{Actor: &p.UserID, Action: "deliverable.downloaded", EntityType: "deliverable",
			EntityID: &d.ID, OrgID: &d.OrgID, After: map[string]any{"file": file, "format": d.Format, "version": d.Version}})
	})
	if err != nil {
		return Download{}, err
	}
	return Download{Name: name, ContentType: ctype, Data: data}, nil
}
