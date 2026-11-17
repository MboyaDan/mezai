package httpapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Stage 17c: uploads are looked at before they are stored. A CV that is a
// PDF with a script, and a request "sample" that is a web page or binary
// renamed to .csv, are refused; plain files still go through.
func TestUploadContentChecks(t *testing.T) {
	e := newEnv(t)
	a := e.signUpAndSignIn("upload.check@sample.invalid")
	for name, c := range map[string]struct {
		data string
		code int
	}{
		"script in PDF":   {"%PDF-1.4\n1 0 obj << /OpenAction << /S /JavaScript /JS (app.alert(1)) >> >> endobj\n%%EOF\n", http.StatusUnprocessableEntity},
		"truncated PDF":   {"%PDF-1.4\n1 0 obj << >> endobj\n", http.StatusUnprocessableEntity},
		"plain PDF still": {string(samplePDF), http.StatusOK},
	} {
		w := e.uploadCV(a, "cv.pdf", []byte(c.data))
		if w.Code != c.code {
			t.Errorf("CV %s: %d %s", name, w.Code, w.Body)
		}
		if c.code != http.StatusOK && !strings.Contains(w.Body.String(), `"field":"cv"`) {
			t.Errorf("CV %s: no field message: %s", name, w.Body)
		}
	}

	client := e.signIn("client.admin@sample.invalid")
	body, _ := json.Marshal(map[string]any{
		"title": "Upload check (Sample)", "task_type": "classification", "brief_md": "Sample.",
		"contact_name": "Sample Client Admin", "contact_email": "client.admin@sample.invalid",
	})
	for name, c := range map[string]struct {
		file, data string
		code       int
	}{
		"web page as csv":  {"chats.csv", "<!DOCTYPE html><script>alert(1)</script>", http.StatusUnprocessableEntity},
		"zip as jsonl":     {"chats.jsonl", "PK\x03\x04\x14\x00\x00\x00", http.StatusUnprocessableEntity},
		"svg as txt":       {"notes.txt", "<svg onload=alert(1)>", http.StatusUnprocessableEntity},
		"plain csv still":  {"chats.csv", "text\nHabari ya asubuhi\n", http.StatusCreated},
		"csv with BOM too": {"chats2.csv", "\xef\xbb\xbftext\nSawa\n", http.StatusCreated},
	} {
		w := e.multipart("/v1/project-requests", map[string]string{"request": string(body)}, "sample", c.file, c.data, client)
		if w.Code != c.code {
			t.Errorf("sample %s: %d %s", name, w.Code, w.Body)
		}
	}
}
