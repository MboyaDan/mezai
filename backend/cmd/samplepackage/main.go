// Command samplepackage writes the public sample delivery (Stage 21b, D-162)
// for the site's Proof page. The data is fictional; the files are made by the
// same code as a real export. Run from backend/: make sample-package.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/MboyaDan/mezai/backend/internal/delivery"
)

func main() {
	out := flag.String("out", "../frontend/public/proof", "directory for the sample files")
	flag.Parse()
	files, err := delivery.SamplePackage()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	for kind, b := range files {
		p := filepath.Join(*out, delivery.SampleFileName(kind))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s (%d bytes)", p, len(b))
	}
}
