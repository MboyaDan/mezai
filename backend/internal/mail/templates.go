// Package mail renders account emails and sends them from the outbox.
package mail

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/MboyaDan/mezai/backend/internal/brand"
)

// Template names stored in email_outbox.template.
const (
	TemplateVerifyEmail   = "verify_email"
	TemplatePasswordReset = "password_reset"
	TemplateInvitation    = "invitation"
	TemplateEnquiry       = "enquiry_received"
)

// Plain text for now; Stage 16 adds HTML versions and preferences.
var templates = map[string]struct{ subject, body string }{
	TemplateVerifyEmail: {
		subject: "Confirm your email address",
		body: `Hello {{.Name}},

Confirm your email address for {{.Brand}} by opening this link:

{{.Link}}

The link works once and expires in 24 hours. If you did not create an account, ignore this email.
`,
	},
	TemplatePasswordReset: {
		subject: "Reset your password",
		body: `Hello {{.Name}},

Someone asked to reset the password for your {{.Brand}} account. To choose a new password, open this link:

{{.Link}}

The link works once and expires in 1 hour. If you did not ask for this, ignore this email; your password has not changed.
`,
	},
	TemplateInvitation: {
		subject: "You are invited to {{.Brand}}",
		body: `Hello,

{{.InvitedBy}} has invited you to {{.Brand}} as {{.Role}}{{if .Org}} for {{.Org}}{{end}}.

To accept and set your password, open this link:

{{.Link}}

The invitation expires in 7 days.
`,
	},
	// To the ops inbox (ENQUIRY_NOTIFY_TO). Reply goes to the enquirer by hand.
	TemplateEnquiry: {
		subject: "New project enquiry: {{.Organisation}}",
		body: `A project enquiry arrived through the {{.Brand}} website.

Name:          {{.Name}}
Organisation:  {{.Organisation}}
Email:         {{.Email}}
Project type:  {{.ProjectType}}
Languages:     {{.Languages}}
Volume:        {{.Volume}}
Timeline:      {{.Timeline}}

Message:
{{.Message}}

Reference: {{.ID}}
It is saved in the enquiries inbox with status "new".
`,
	},
}

// Rendered is a ready-to-send message.
type Rendered struct {
	Subject, Body string
}

// Render fills a template with data from the outbox row.
func Render(name string, data map[string]string) (Rendered, error) {
	t, ok := templates[name]
	if !ok {
		return Rendered{}, fmt.Errorf("mail: unknown template %q", name)
	}
	vars := map[string]string{"Brand": brand.Name}
	for k, v := range data {
		vars[k] = v
	}
	render := func(src string) (string, error) {
		tpl, err := template.New(name).Option("missingkey=error").Parse(src)
		if err != nil {
			return "", err
		}
		var b bytes.Buffer
		if err := tpl.Execute(&b, vars); err != nil {
			return "", err
		}
		return b.String(), nil
	}
	subject, err := render(t.subject)
	if err != nil {
		return Rendered{}, err
	}
	body, err := render(t.body)
	if err != nil {
		return Rendered{}, err
	}
	body += "\n-- \n" + brand.Name + ", " + brand.Descriptor + "\n"
	return Rendered{Subject: subject, Body: body}, nil
}
