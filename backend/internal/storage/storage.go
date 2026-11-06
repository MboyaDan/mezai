// Package storage talks to S3-compatible object storage (SeaweedFS locally,
// Cloudflare R2 or Supabase Storage in staging).
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Options struct {
	Endpoint        string // empty means the AWS default endpoint
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

type Store struct {
	client *s3.Client
	bucket string
}

func New(o Options) *Store {
	so := s3.Options{
		Region:       o.Region,
		UsePathStyle: o.UsePathStyle,
		Credentials:  credentials.NewStaticCredentialsProvider(o.AccessKeyID, o.SecretAccessKey, ""),
	}
	if o.Endpoint != "" {
		so.BaseEndpoint = aws.String(o.Endpoint)
	}
	return &Store{client: s3.New(so), bucket: o.Bucket}
}

// Ping confirms the bucket exists and the credentials can reach it.
func (s *Store) Ping(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		return fmt.Errorf("head bucket %q: %w", s.bucket, err)
	}
	return nil
}

// Put stores an object. Keys are opaque paths chosen by the API, never by users.
func (s *Store) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
		Body: bytes.NewReader(data), ContentType: aws.String(contentType),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("put %q: %w", key, err)
	}
	return nil
}

// Get reads an object, refusing anything larger than max bytes.
func (s *Store) Get(ctx context.Context, key string, max int64) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", key, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("get %q: larger than %d bytes", key, max)
	}
	return data, nil
}

// Blobs is what handlers need from object storage; Memory implements it for tests.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string, max int64) ([]byte, error)
}

// Memory is an in-process Blobs for tests and for running without storage.
type Memory struct {
	mu sync.Mutex
	m  map[string][]byte
}

func NewMemory() *Memory { return &Memory{m: map[string][]byte{}} }

func (m *Memory) Put(_ context.Context, key, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = bytes.Clone(data)
	return nil
}

func (m *Memory) Get(_ context.Context, key string, max int64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.m[key]
	if !ok {
		return nil, fmt.Errorf("get %q: not found", key)
	}
	if int64(len(d)) > max {
		return nil, fmt.Errorf("get %q: larger than %d bytes", key, max)
	}
	return bytes.Clone(d), nil
}

// Corrupt replaces an object, for tests that prove digests are checked.
func (m *Memory) Corrupt(key string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = data
}
