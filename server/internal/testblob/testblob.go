// Package testblob gives integration tests a real, empty S3 bucket. Tests are
// skipped unless TEST_S3_ENDPOINT points at an S3-compatible server, e.g. the
// local versitygw from docker-compose:
//
//	TEST_S3_ENDPOINT=http://localhost:7070
//	TEST_S3_ACCESS_KEY=rift  TEST_S3_SECRET_KEY=rift-dev-secret
package testblob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"rift/server/internal/blob"
)

func config(t testing.TB) blob.Config {
	t.Helper()
	endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_S3_ENDPOINT not set; skipping storage integration test")
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return blob.Config{
		Endpoint:  endpoint,
		Region:    "us-east-1",
		Bucket:    "rift-test-" + hex.EncodeToString(b),
		AccessKey: getenv("TEST_S3_ACCESS_KEY", "rift"),
		SecretKey: getenv("TEST_S3_SECRET_KEY", "rift-dev-secret"),
	}
}

// New creates a fresh bucket and empties and deletes it when the test ends.
func New(t testing.TB) *blob.S3 {
	t.Helper()
	cfg := config(t)
	store := blob.NewS3(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}
	t.Cleanup(func() { destroy(cfg) })
	return store
}

func destroy(cfg blob.Config) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})
	if ups, err := c.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{Bucket: &cfg.Bucket}); err == nil {
		for _, u := range ups.Uploads {
			_, _ = c.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: &cfg.Bucket, Key: u.Key, UploadId: u.UploadId})
		}
	}
	if objs, err := c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: &cfg.Bucket}); err == nil {
		for _, o := range objs.Contents {
			_, _ = c.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &cfg.Bucket, Key: o.Key})
		}
	}
	_, _ = c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: &cfg.Bucket})
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
