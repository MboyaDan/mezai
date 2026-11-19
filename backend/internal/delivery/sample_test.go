package delivery

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The site serves the committed sample files; they must be what the export
// code makes today. Regenerate with `make sample-package` after a change.
func TestSamplePackageCommitted(t *testing.T) {
	files, err := SamplePackage()
	if err != nil {
		t.Fatal(err)
	}
	for kind, want := range files {
		p := filepath.Join("..", "..", "..", "frontend", "public", "proof", SampleFileName(kind))
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: %v (run make sample-package)", kind, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from the export code's output; run make sample-package", p)
		}
	}
}

func TestSamplePackageIsSample(t *testing.T) {
	files, err := SamplePackage()
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(files["manifest"], &m); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(m.Project.Name, "(Sample)") || !strings.HasSuffix(m.Project.Organisation, "(Sample)") {
		t.Fatalf("sample names must say (Sample): %+v", m.Project)
	}
	if m.Items.Delivered != 12 || m.Items.Rejected != 1 || !m.Items.Complete || m.Items.Adjudicated != 2 {
		t.Fatalf("items: %+v", m.Items)
	}
	// Checksums in the manifest are those of the files beside it.
	for i, kind := range []string{"jsonl", "csv"} {
		if m.Files[i].SHA256 != checksum(files[kind]) || m.Files[i].Bytes != len(files[kind]) {
			t.Errorf("%s checksum or size wrong in manifest", kind)
		}
	}
	if len(m.Agreement) == 0 || m.GoldAccuracy.Accuracy == nil {
		t.Fatalf("agreement and gold accuracy expected: %+v %+v", m.Agreement, m.GoldAccuracy)
	}
}
