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
	TemplateVerifyEmail    = "verify_email"
	TemplatePasswordReset  = "password_reset"
	TemplateInvitation     = "invitation"
	TemplateEnquiry        = "enquiry_received"
	TemplateProjectRequest = "project_request_received"
	TemplateTrustPause     = "trust_pause"
)

// Plain text (D-141): readable in every client and on a basic phone. Render
// also builds a branded HTML version from the same text (D-155, html.go);
// action labels the button for the first link on its own line.
var templates = map[string]struct{ subject, action, body string }{
	TemplateVerifyEmail: {
		subject: "Confirm your email address",
		action:  "Confirm email address",
		body: `Hello {{.Name}},

Confirm your email address for {{.Brand}} by opening this link:

{{.Link}}

The link works once and expires in 24 hours. If you did not create an account, ignore this email.
`,
	},
	TemplatePasswordReset: {
		subject: "Reset your password",
		action:  "Choose a new password",
		body: `Hello {{.Name}},

Someone asked to reset the password for your {{.Brand}} account. To choose a new password, open this link:

{{.Link}}

The link works once and expires in 1 hour. If you did not ask for this, ignore this email; your password has not changed.
`,
	},
	TemplateInvitation: {
		subject: "You are invited to {{.Brand}}",
		action:  "Accept invitation",
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
	// To the ops inbox when a contributor is paused automatically (brief 8.4).
	TemplateTrustPause: {
		subject: "Contributor {{.Code}} paused automatically",
		action:  "Open contributor",
		body: `{{.Code}} was paused from new tasks because their trust score fell below the floor.

Trust score:        {{.Score}} (floor {{.Floor}})
Gold accuracy:      {{.Gold}}
Agreement:          {{.Agreement}}
Review acceptance:  {{.Review}}

Review their work and reinstate them with a note if the pause is not deserved:
{{.Link}}
`,
	},
	TemplateProjectRequest: {
		subject: "Project request from {{.Organisation}}: {{.Title}}",
		action:  "Open the request",
		body: `{{.Organisation}} submitted a project request in the {{.Brand}} workspace.

Title:      {{.Title}}
Task type:  {{.TaskType}}
Sample:     {{.Sample}}

Open it to convert it into a draft project or decline it:
{{.Link}}

Reference: {{.ID}}
`,
	},
	// Contributor notifications (Stage 16). {{.Origin}} is added by the worker.
	TemplateQualificationResult: {
		subject: "Your {{.Test}} result",
		action:  "Open qualification",
		body: `Hello {{.Name}},

{{if eq .Passed "yes"}}You passed the {{.Test}}. Projects that need this skill can now offer you tasks.{{else}}You did not pass the {{.Test}} this time.{{end}}
{{- if .Note}}

Note from the operations team:
{{.Note}}
{{- end}}

Your tests page shows your result and when you can try again:
{{.Origin}}/c/qualify
`,
	},
	TemplateProjectAvailable: {
		subject: "New project open to you: {{.Project}}",
		action:  "Open the project",
		body: `Hello {{.Name}},

A new project you qualify for is open:

{{.Project}}
Task type:  {{.TaskType}}
Pay:        {{.Pay}} for each accepted task
{{- if .Deadline}}
Open until: {{.Deadline}}{{end}}

Read the brief and start when you are ready:
{{.Origin}}/c/projects/{{.ProjectID}}

You get this email because "New projects" is on. Change it on your profile:
{{.Origin}}/c/profile#notifications
`,
	},
	TemplateFeedbackReceived: {
		subject: "New feedback on your work",
		action:  "Read the feedback",
		body: `Hello {{.Name}},

A reviewer left feedback on your work in {{.Project}}. Read it on your dashboard:
{{.Origin}}/c

We send at most one of these emails a day, however much feedback arrives.

You get this email because "Feedback" is on. Change it on your profile:
{{.Origin}}/c/profile#notifications
`,
	},
	TemplatePayoutSent: {
		subject: "Payout sent: {{.Amount}}",
		action:  "Open your statement",
		body: `Hello {{.Name}},

{{.Amount}} was sent to your M-Pesa number ending {{.PhoneEnd}}.

M-Pesa receipt:  {{.Receipt}}
Recorded on:     {{.Date}}

Your earnings statement shows this payout and your balance:
{{.Origin}}/c/earnings

If the money has not arrived, reply to the operations team with the receipt above.
`,
	},
}

// Rendered is a ready-to-send message.
type Rendered struct {
	Subject, Body string
	// HTML is the branded version of Body (D-155); empty sends text only.
	HTML string
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
	action := t.action
	if action == "" {
		action = "Open " + brand.Name
	}
	page := renderHTML(subject, body, action, vars["Origin"])
	body += "\n-- \n" + brand.Name + ", " + brand.Descriptor + "\n"
	return Rendered{Subject: subject, Body: body, HTML: page}, nil
}
