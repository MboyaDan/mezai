package mail

import (
	"strings"
	"testing"
)

// D-158: every branch of the screening emails renders, says the right thing,
// and gets a button to the site.
func TestScreeningTemplates(t *testing.T) {
	o := "https://site.example"
	cases := []struct {
		tpl     string
		data    map[string]string
		subject string
		want    []string
		not     []string
	}{
		{TemplateScreeningDecision, map[string]string{"Decision": "shortlisted", "Note": ""}, "Your application is shortlisted",
			[]string{"core check", "phone interview", o + "/c"}, []string{"Note from", "CV"}},
		{TemplateScreeningDecision, map[string]string{"Decision": "waitlisted", "Note": "No Kiswahili projects this month."}, "Your application is on our waiting list",
			[]string{"waiting list", "No Kiswahili projects this month."}, []string{"CV"}},
		{TemplateScreeningDecision, map[string]string{"Decision": "declined", "Note": "Not enough experience yet."}, "About your application",
			[]string{"not taking your application further", "Not enough experience yet.", "CV, it has been deleted"}, nil},
		{TemplateInterviewResult, map[string]string{"Outcome": "passed", "RetryDays": "30"}, "You passed your interview", []string{"passed"}, []string{"30 days"}},
		{TemplateInterviewResult, map[string]string{"Outcome": "failed", "RetryDays": "30"}, "Your interview result", []string{"after 30 days"}, nil},
		{TemplateInterviewResult, map[string]string{"Outcome": "no_answer", "RetryDays": "30"}, "We could not reach you by phone", []string{"phone number on your profile"}, nil},
		{TemplateIdentityDecision, map[string]string{"Decision": "verified", "Note": ""}, "Your identity is verified", []string{o + "/c/profile"}, []string{"Reason"}},
		{TemplateIdentityDecision, map[string]string{"Decision": "rejected", "Note": "The ID number does not match."}, "We could not verify your identity",
			[]string{"The ID number does not match.", "check them again"}, nil},
		{TemplateActivated, nil, "You can start tasks on MezAI", []string{"active MezAI contributor", o + "/c/work"}, nil},
		{TemplateAccountExists, map[string]string{"Link": o + "/login", "ResetLink": o + "/forgot-password"}, "You already have a MezAI account",
			[]string{"nothing was created", o + "/forgot-password"}, nil},
	}
	for _, c := range cases {
		data := map[string]string{"Name": "Sample Contributor 014", "Origin": o}
		for k, v := range c.data {
			data[k] = v
		}
		r, err := Render(c.tpl, data)
		if err != nil {
			t.Fatalf("%s %v: %v", c.tpl, c.data, err)
		}
		if r.Subject != c.subject {
			t.Errorf("%s %v: subject %q", c.tpl, c.data, r.Subject)
		}
		for _, w := range c.want {
			if !strings.Contains(r.Body, w) {
				t.Errorf("%s %v: body lacks %q:\n%s", c.tpl, c.data, w, r.Body)
			}
		}
		for _, w := range c.not {
			if strings.Contains(r.Body, w) {
				t.Errorf("%s %v: body has %q", c.tpl, c.data, w)
			}
		}
		if !strings.Contains(r.HTML, "background:#c8f031") {
			t.Errorf("%s %v: no button", c.tpl, c.data)
		}
		if strings.Contains(r.Body, "\n\n\n") {
			t.Errorf("%s %v: stray blank lines:\n%s", c.tpl, c.data, r.Body)
		}
	}
}
