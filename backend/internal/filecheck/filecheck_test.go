package filecheck

import (
	"errors"
	"testing"
)

const plainPDF = "%PDF-1.4\n1 0 obj << /Type /Catalog /Pages 2 0 R /OpenAction 3 0 R >> endobj\n" +
	"4 0 obj << /Length 5 /Filter /FlateDecode /JSONish 1 >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n"

func TestPDF(t *testing.T) {
	for name, c := range map[string]struct {
		data string
		want error
	}{
		"plain document":      {plainPDF, nil},
		"not a PDF":           {"<html>%%EOF", ErrNotPDF},
		"truncated":           {"%PDF-1.4\n1 0 obj << >> endobj\n", ErrNotPDF},
		"script":              {"%PDF-1.4\n<< /S /JavaScript /JS (app.alert(1)) >>\n%%EOF", ErrActivePDF},
		"short script name":   {"%PDF-1.4\n<< /JS 5 0 R >>\n%%EOF", ErrActivePDF},
		"launch action":       {"%PDF-1.4\n<< /S /Launch /F (cmd.exe) >>\n%%EOF", ErrActivePDF},
		"embedded file":       {"%PDF-1.4\n<< /Type /EmbeddedFile >>\n%%EOF", ErrActivePDF},
		"name at end of file": {"%PDF-1.4\n%%EOF\n/JS", ErrActivePDF},
	} {
		if got := PDF([]byte(c.data)); !errors.Is(got, c.want) {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}

func TestText(t *testing.T) {
	for name, c := range map[string]struct {
		data string
		want error
	}{
		"csv":              {"text,label\nHabari ya asubuhi,greeting\n", nil},
		"jsonl with bom":   {"\xef\xbb\xbf{\"text\":\"Sawa\"}\n", nil},
		"text with angles": {"a < b and c > d\n", nil},
		"binary":           {"PK\x03\x04\x00\x00", ErrNotText},
		"invalid utf-8":    {"caf\xe9\n", ErrNotText},
		"html page":        {"  <!DOCTYPE html><html><script>x</script>", ErrMarkupInText},
		"svg":              {"<svg onload=alert(1)>", ErrMarkupInText},
		"xml":              {"<?xml version=\"1.0\"?><a/>", ErrMarkupInText},
	} {
		if got := Text([]byte(c.data)); !errors.Is(got, c.want) {
			t.Errorf("%s: got %v, want %v", name, got, c.want)
		}
	}
}
