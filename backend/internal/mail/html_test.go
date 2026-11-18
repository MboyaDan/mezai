package mail

import (
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
)

func TestHTMLHasLogoAndButton(t *testing.T) {
	r, err := Render(TemplateVerifyEmail, map[string]string{
		"Name": "Sample", "Link": "https://site.example/verify-email?token=a&b", "Origin": "https://site.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<img src="https://site.example` + LogoPath + `"`,
		`alt="MezAI"`,
		`href="https://site.example/verify-email?token=a&amp;b"`,
		`>Confirm email address</a>`,
		`MezAI, a division of ELS Software Solutions`,
		`<title>Confirm your email address</title>`,
	} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}
	if !strings.Contains(r.Body, "https://site.example/verify-email?token=a&b") {
		t.Error("the text version must keep the raw link")
	}
}

func TestHTMLWithoutOriginUsesWordmark(t *testing.T) {
	r, err := Render(TemplateVerifyEmail, map[string]string{"Name": "Sample", "Link": "http://x/v?token=t"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.HTML, "<img") || strings.Contains(r.HTML, `href="http://x/`) {
		t.Fatal("no origin: no image and no links (nothing to call our own)")
	}
	if !strings.Contains(r.HTML, ">MezAI</span>") {
		t.Fatal("wordmark expected")
	}
}

// Enquiry fields come from the public: everything is escaped and only links
// to our own site become clickable.
func TestHTMLEscapesPublicInput(t *testing.T) {
	r, err := Render(TemplateEnquiry, map[string]string{
		"Origin": "https://site.example", "Name": `<b>Eve</b>`, "Organisation": `"Acme" & Co`,
		"Email": "eve@example.com", "ProjectType": "x", "Languages": "x", "Volume": "x", "Timeline": "x",
		"Message": "<script>alert(1)</script>\nPay here: https://evil.example/login\n\nhttps://evil.example/button\nhttps://site.example/w/enquiries",
		"ID":      "id",
	})
	if err != nil {
		t.Fatal(err)
	}
	h := r.HTML
	for _, bad := range []string{"<script", "<b>Eve", `href="https://evil.example`, `"Acme" & Co`} {
		if strings.Contains(h, bad) {
			t.Errorf("HTML contains %q", bad)
		}
	}
	for _, want := range []string{"&lt;script&gt;", "&lt;b&gt;Eve&lt;/b&gt;", "https://evil.example/login", "&#34;Acme&#34; &amp; Co", `href="https://site.example/w/enquiries"`} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}
	if strings.Contains(h, "background:#c8f031") {
		t.Error("an address typed by the public must never become a button")
	}
}

func TestMessageIsMultipartAlternative(t *testing.T) {
	long := strings.Repeat("word ", 400) // one 2,000-character line
	r, err := Render(TemplateEnquiry, map[string]string{
		"Origin": "https://site.example", "Name": "Ann", "Organisation": "Org", "Email": "a@example.com",
		"ProjectType": "x", "Languages": "Kiswahili", "Volume": "x", "Timeline": "x", "Message": long, "ID": "id",
	})
	if err != nil {
		t.Fatal(err)
	}
	from, _ := mail.ParseAddress("MezAI <no-reply@site.example>")
	to, _ := mail.ParseAddress("ops@site.example")
	raw := message(from, to, r)

	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line of %d characters breaks SMTP", len(line))
		}
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type %q, %v", mt, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	var texts []string
	for {
		p, err := mr.NextPart() // decodes quoted-printable
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		types = append(types, strings.Split(p.Header.Get("Content-Type"), ";")[0])
		texts = append(texts, string(b))
	}
	if strings.Join(types, ",") != "text/plain,text/html" {
		t.Fatalf("parts %v: text must come first, HTML last", types)
	}
	if !strings.Contains(texts[0], strings.TrimSpace(long)) || !strings.Contains(texts[1], "Kiswahili") {
		t.Fatal("parts did not decode to their content")
	}
}

func TestMessageTextOnlyWithoutHTML(t *testing.T) {
	from, _ := mail.ParseAddress("no-reply@site.example")
	to, _ := mail.ParseAddress("a@site.example")
	raw := string(message(from, to, Rendered{Subject: "s", Body: "hello\n"}))
	if !strings.Contains(raw, "Content-Type: text/plain; charset=utf-8") || strings.Contains(raw, "multipart") {
		t.Fatal(raw)
	}
}
