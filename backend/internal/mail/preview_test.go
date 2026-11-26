package mail

import (
	"os"
	"path/filepath"
	"testing"
)

// Writes sample emails as HTML for a visual check: MAIL_PREVIEW_DIR=dir go test -run Preview
func TestPreview(t *testing.T) {
	dir := os.Getenv("MAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set MAIL_PREVIEW_DIR to write previews")
	}
	origin := os.Getenv("MAIL_PREVIEW_ORIGIN")
	samples := map[string]map[string]string{
		TemplateVerifyEmail: {"Name": "Sample Contributor 014", "Link": origin + "/verify-email?token=sample"},
		TemplateEnquiry: {"Name": "Sample Person", "Organisation": "Sample Client Ltd (Sample)", "Email": "person@sample.invalid",
			"ProjectType": "African language data", "Languages": "Kiswahili, Dholuo", "Volume": "1,000 to 10,000 items",
			"Timeline": "In one to three months", "Message": "We would like to label customer messages.", "ID": "01a1f03d-sample",
			"Link": origin + "/w/enquiries/01a1f03d-sample"},
		TemplatePayoutSent:        {"Name": "Sample Contributor 014", "Amount": "KES 1,250.00", "Receipt": "SAMPLE0001", "Date": "18 Nov 2026", "PhoneEnd": "000"},
		TemplateActivated:         {"Name": "Sample Contributor 014"},
		TemplateScreeningDecision: {"Name": "Sample Contributor 014", "Decision": "shortlisted", "Note": ""},
		TemplateAccountExists:     {"Name": "Sample Contributor 014", "Link": origin + "/login", "ResetLink": origin + "/forgot-password"},
	}
	for name, data := range samples {
		data["Origin"] = origin
		r, err := Render(name, data)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".html"), []byte(r.HTML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
