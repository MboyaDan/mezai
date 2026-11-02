// Package storage talks to S3-compatible object storage (SeaweedFS locally,
// Cloudflare R2 or Supabase Storage in staging).
package storage

import (
	"context"
	"fmt"

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
