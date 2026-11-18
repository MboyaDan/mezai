package mail

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/MboyaDan/mezai/backend/internal/brand"
)

// HTML emails (D-155). Every template stays plain text (D-141); the HTML part
// is built from that text, so the two never say different things and a basic
// phone still gets the readable version. Everything from the text is escaped:
// enquiry fields and names come from the public.

// LogoPath is the email logo under the site origin (frontend/public). Gmail and
// Outlook do not show SVG, so it is a PNG (frontend/scripts/email-logo.mjs).
const LogoPath = "/brand/email/mezai-logo-email.png"

// Logo size in the email, in CSS pixels (the file is three times larger).
const logoWidth, logoHeight = 132, 45

// Colours from DESIGN.md (globals.css).
const (
	colPage   = "#efebe1"
	colPaper  = "#fbf9f4"
	colInk    = "#151714"
	colMuted  = "#5a5f55"
	colBorder = "#e2ddd1"
	colLime   = "#c8f031"
)

const fontStack = `-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif`

var (
	urlLine  = regexp.MustCompile(`^https?://\S+$`)
	urlInner = regexp.MustCompile(`https?://[^\s<>"]+`)
)

// renderHTML wraps a rendered text body in the branded layout. action labels
// the first link that stands on its own line; later ones stay text links.
// Only links to the site itself (origin) become clickable: an address typed
// into a public form stays plain text, so an email can never carry a
// lookalike button.
func renderHTML(subject, text, action, origin string) string {
	origin = strings.TrimRight(origin, "/")
	own := func(u string) bool {
		return origin != "" && (u == origin || strings.HasPrefix(u, origin+"/") || strings.HasPrefix(u, origin+"?"))
	}
	var body strings.Builder
	buttoned := false
	for _, para := range strings.Split(strings.TrimSpace(text), "\n\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if urlLine.MatchString(para) && own(para) {
			u := html.EscapeString(para)
			if !buttoned {
				buttoned = true
				fmt.Fprintf(&body, `<p style="margin:8px 0 6px"><a href="%s" style="display:inline-block;background:%s;color:%s;font-weight:600;text-decoration:none;padding:12px 22px;border-radius:8px">%s</a></p>`+"\n",
					u, colLime, colInk, html.EscapeString(action))
				fmt.Fprintf(&body, `<p style="margin:0 0 18px;font-size:13px;line-height:1.5;color:%s;word-break:break-all">Or paste this link into your browser:<br><a href="%s" style="color:%s">%s</a></p>`+"\n",
					colMuted, u, colMuted, u)
			} else {
				fmt.Fprintf(&body, `<p style="margin:0 0 18px;word-break:break-all"><a href="%s" style="color:%s">%s</a></p>`+"\n", u, colInk, u)
			}
			continue
		}
		lines := strings.Split(para, "\n")
		for i, l := range lines {
			lines[i] = linkify(strings.TrimRight(l, " "), own)
		}
		fmt.Fprintf(&body, `<p style="margin:0 0 18px">%s</p>`+"\n", strings.Join(lines, "<br>\n"))
	}

	logo := fmt.Sprintf(`<span style="font-size:22px;font-weight:700;color:%s;letter-spacing:-0.01em">%s</span>`, colPaper, html.EscapeString(brand.Name))
	if strings.HasPrefix(origin, "https://") || strings.HasPrefix(origin, "http://") {
		logo = fmt.Sprintf(`<img src="%s" width="%d" height="%d" alt="%s" style="display:block;border:0;outline:none;width:%dpx;height:%dpx">`,
			html.EscapeString(strings.TrimRight(origin, "/")+LogoPath), logoWidth, logoHeight, html.EscapeString(brand.Name), logoWidth, logoHeight)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background:%s">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background:%s">
<tr><td align="center" style="padding:28px 12px">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;background:%s;border:1px solid %s;border-radius:12px;overflow:hidden">
<tr><td style="background:%s;padding:22px 28px">%s</td></tr>
<tr><td style="padding:30px 28px 12px;font-family:%s;font-size:16px;line-height:1.6;color:%s">
%s</td></tr>
<tr><td style="padding:18px 28px 24px;border-top:1px solid %s;font-family:%s;font-size:13px;line-height:1.5;color:%s">%s, %s</td></tr>
</table>
</td></tr>
</table>
</body>
</html>
`,
		html.EscapeString(subject), colPage, colPage, colPaper, colBorder,
		colInk, logo,
		fontStack, colInk, body.String(),
		colBorder, fontStack, colMuted, html.EscapeString(brand.Name), html.EscapeString(brand.Descriptor))
	return b.String()
}

// linkify escapes a line of text and turns the site's own addresses in it
// into links.
func linkify(line string, own func(string) bool) string {
	var b strings.Builder
	last := 0
	for _, m := range urlInner.FindAllStringIndex(line, -1) {
		raw := strings.TrimRight(line[m[0]:m[1]], ".,;:)")
		if !own(raw) {
			continue
		}
		b.WriteString(html.EscapeString(line[last:m[0]]))
		u := html.EscapeString(raw)
		fmt.Fprintf(&b, `<a href="%s" style="color:%s">%s</a>`, u, colInk, u)
		last = m[0] + len(raw)
	}
	b.WriteString(html.EscapeString(line[last:]))
	return b.String()
}
