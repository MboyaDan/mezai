package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Sender delivers one message.
type Sender interface {
	Send(ctx context.Context, to string, m Rendered) error
}

// SMTP sends through an SMTP server: mailpit locally, a provider in staging.
// STARTTLS is used when the server offers it; credentials are only sent over TLS.
type SMTP struct {
	Addr     string // host:port
	From     string // "Name <address>"
	Username string
	Password string
}

func (s SMTP) Send(ctx context.Context, to string, m Rendered) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("MAIL_FROM: %w", err)
	}
	rcpt, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, host) // refuses to send credentials without TLS
	}
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(s.Addr, auth, from.Address, []string{rcpt.Address}, message(from, rcpt, m))
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func message(from, to *mail.Address, m Rendered) []byte {
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\n")
	if m.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
		b.WriteString(crlf(m.Body))
		return []byte(b.String())
	}
	// Text first, HTML last: clients show the last part they can display.
	boundary := "mezai-" + hex.EncodeToString(id)
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
	b.WriteString(qp(m.Body))
	fmt.Fprintf(&b, "\r\n--%s\r\nContent-Type: text/html; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
	b.WriteString(qp(m.HTML))
	fmt.Fprintf(&b, "\r\n--%s--\r\n", boundary)
	return []byte(b.String())
}

func crlf(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }

// qp encodes a part as quoted-printable, which keeps every line under SMTP's
// length limit whatever the text holds.
func qp(s string) string {
	var buf strings.Builder
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(crlf(s)))
	_ = w.Close()
	return buf.String()
}
