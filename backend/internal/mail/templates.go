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
	// To the owner when someone signs up with their address (D-158).
	TemplateAccountExists = "account_exists"
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
	TemplateAccountExists: {
		subject: "You already have a {{.Brand}} account",
		action:  "Sign in",
		body: `Hello {{.Name}},

Someone tried to create a new {{.Brand}} account with this email address. You already have one, so nothing was created.

If it was you, sign in instead:

{{.Link}}

Forgotten your password? Choose a new one here:
{{.ResetLink}}

If it was not you, ignore this email. Your account has not changed.
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
	// Screening steps (Stage 19d, D-158). Notes are what Ops typed for the
	// contributor to read; interview notes stay internal and are never sent.
	TemplateScreeningDecision: {
		subject: `{{if eq .Decision "shortlisted"}}Your application is shortlisted{{else if eq .Decision "waitlisted"}}Your application is on our waiting list{{else}}About your application{{end}}`,
		action:  "Open your dashboard",
		body: `Hello {{.Name}},

{{if eq .Decision "shortlisted"}}Good news: your application to contribute to {{.Brand}} is shortlisted.

The next steps are the core check on your tests page, a short phone interview on the number on your profile, and an identity check. Your dashboard shows which are still open.
{{- else if eq .Decision "waitlisted"}}Thank you for applying to contribute to {{.Brand}}. We have more applicants than work for your skills at the moment, so your application is on our waiting list.

You do not need to do anything. We will email you if this changes.
{{- else}}Thank you for applying to contribute to {{.Brand}}. We are not taking your application further.
{{- end}}
{{- if .Note}}

Note from the operations team:
{{.Note}}
{{- end}}
{{- if eq .Decision "declined"}}

If you uploaded a CV, it has been deleted.
{{- end}}

{{.Origin}}/c
`,
	},
	TemplateInterviewResult: {
		subject: `{{if eq .Outcome "passed"}}You passed your interview{{else if eq .Outcome "failed"}}Your interview result{{else}}We could not reach you by phone{{end}}`,
		action:  "Open your dashboard",
		body: `Hello {{.Name}},

{{if eq .Outcome "passed"}}You passed your {{.Brand}} phone interview. Your dashboard shows any steps still open.
{{- else if eq .Outcome "failed"}}Thank you for taking the time to speak with us. You did not pass the phone interview this time. You can be interviewed again after {{.RetryDays}} days.
{{- else}}We phoned you for your {{.Brand}} interview but could not reach you. We will try again. Check that the phone number on your profile is right.
{{- end}}

{{.Origin}}/c
`,
	},
	TemplateIdentityDecision: {
		subject: `{{if eq .Decision "verified"}}Your identity is verified{{else}}We could not verify your identity{{end}}`,
		action:  "Open your profile",
		body: `Hello {{.Name}},

{{if eq .Decision "verified"}}Your identity check for {{.Brand}} is complete. Your dashboard shows any steps still open.
{{- else}}We could not verify your identity for {{.Brand}}.

Reason from the operations team:
{{.Note}}

Correct the details on your profile and we will check them again.
{{- end}}

{{.Origin}}/c/profile
`,
	},
	TemplateActivated: {
		subject: "You can start tasks on {{.Brand}}",
		action:  "See your tasks",
		body: `Hello {{.Name}},

Every step is complete: you are now an active {{.Brand}} contributor.

Projects that match your skills appear on your tasks page. Read each project's brief before you start; your first tasks help you learn the guidelines.

{{.Origin}}/c/work

Your trust score and earnings appear on your dashboard as your work is reviewed.
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
