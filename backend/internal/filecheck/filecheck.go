// Package filecheck looks inside uploaded files before they are stored
// (Stage 17c). It is a cheap filter on top of size limits, attachment-only
// downloads and a deny-all content policy, not a malware scanner: a PDF can
// hide names inside compressed object streams, which this does not unpack.
package filecheck

import (
	"bytes"
	"errors"
	"unicode/utf8"
)

var (
	ErrNotPDF       = errors.New("filecheck: not a PDF")
	ErrActivePDF    = errors.New("filecheck: PDF holds scripts, launch actions or embedded files")
	ErrNotText      = errors.New("filecheck: not UTF-8 text")
	ErrMarkupInText = errors.New("filecheck: looks like a web page or script, not data")
)

// activeNames are PDF dictionary names for content that runs or carries
// other files. Plain documents (CVs, invoices) never need them.
var activeNames = [][]byte{
	[]byte("/JavaScript"), []byte("/JS"), []byte("/Launch"), []byte("/EmbeddedFile"),
	[]byte("/RichMedia"), []byte("/XFA"), []byte("/SubmitForm"), []byte("/ImportData"),
}

// PDF accepts a file that starts like a PDF, ends with an end-of-file marker
// and names no active content.
func PDF(data []byte) error {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return ErrNotPDF
	}
	tail := data[max(0, len(data)-1024):]
	if !bytes.Contains(tail, []byte("%%EOF")) {
		return ErrNotPDF
	}
	for _, n := range activeNames {
		for i := 0; ; {
			j := bytes.Index(data[i:], n)
			if j < 0 {
				break
			}
			end := i + j + len(n)
			// A whole name only: "/JS" must not match "/JSON" or "/JSomething".
			if end == len(data) || !isNameChar(data[end]) {
				return ErrActivePDF
			}
			i = end
		}
	}
	return nil
}

func isNameChar(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '.' || b == '-'
}

// Text accepts UTF-8 text with no NUL bytes that does not open like an HTML
// page or SVG (a data sample never does). A UTF-8 byte order mark is allowed.
func Text(data []byte) error {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return ErrNotText
	}
	head := bytes.ToLower(bytes.TrimSpace(data[:min(len(data), 512)]))
	for _, p := range [][]byte{[]byte("<!doctype"), []byte("<html"), []byte("<script"), []byte("<svg"), []byte("<?xml")} {
		if bytes.HasPrefix(head, p) {
			return ErrMarkupInText
		}
	}
	return nil
}
