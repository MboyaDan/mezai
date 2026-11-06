package storage_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/storage"
)

// Round trip against real S3-compatible storage. Runs when S3_TEST_ENDPOINT
// is set (for example http://localhost:8333 with `make up`); the in-memory
// store covers the API tests.
func TestS3RoundTrip(t *testing.T) {
	ep := os.Getenv("S3_TEST_ENDPOINT")
	if ep == "" {
		t.Skip("S3_TEST_ENDPOINT not set")
	}
	s := storage.New(storage.Options{Endpoint: ep, Region: "us-east-1", Bucket: os.Getenv("S3_BUCKET"),
		AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), UsePathStyle: true})
	ctx := context.Background()
	data := []byte("text\nHabari\n")
	if err := s.Put(ctx, "test/roundtrip.csv", "application/octet-stream", data); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "test/roundtrip.csv", 1<<20)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("get = %q, %v", got, err)
	}
	if _, err := s.Get(ctx, "test/roundtrip.csv", 4); err == nil {
		t.Fatal("size limit not enforced")
	}
}

func TestMemory(t *testing.T) {
	m := storage.NewMemory()
	ctx := context.Background()
	_ = m.Put(ctx, "k", "", []byte("abc"))
	if got, err := m.Get(ctx, "k", 3); err != nil || string(got) != "abc" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := m.Get(ctx, "k", 2); err == nil {
		t.Fatal("size limit not enforced")
	}
	if _, err := m.Get(ctx, "missing", 2); err == nil {
		t.Fatal("missing key returned data")
	}
}
