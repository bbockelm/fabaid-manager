package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"

	"github.com/bbockelm/fabaid-manager/internal/config"
)

// Store provides S3-compatible object storage operations.
type Store struct {
	client *minio.Client
	bucket string
}

// New creates a new S3 storage client.
func New(cfg *config.Config) (*Store, error) {
	endpoint := cfg.S3Endpoint
	// Strip scheme for minio client
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	// Ensure bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.S3Bucket)
	if err != nil {
		return nil, fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.S3Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("creating bucket: %w", err)
		}
		log.Info().Str("bucket", cfg.S3Bucket).Msg("Created S3 bucket")
	}

	log.Info().Str("endpoint", endpoint).Str("bucket", cfg.S3Bucket).Msg("Connected to S3 storage")

	return &Store{
		client: client,
		bucket: cfg.S3Bucket,
	}, nil
}

// Upload stores a file in S3.
func (s *Store) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("uploading to S3: %w", err)
	}
	return nil
}

// Download retrieves a file from S3.
func (s *Store) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("downloading from S3: %w", err)
	}
	return obj, nil
}

// Delete removes a file from S3.
func (s *Store) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("deleting from S3: %w", err)
	}
	return nil
}

// GenerateKey creates a unique S3 key for a document.
func GenerateKey(entityType, entityID, filename string) string {
	safeName := url.PathEscape(filename)
	return fmt.Sprintf("%s/%s/%s", entityType, entityID, safeName)
}

// ListKeys returns all object keys in the bucket (used for backup).
func (s *Store) ListKeys(ctx context.Context) ([]string, error) {
	var keys []string
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("listing objects: %w", obj.Err)
		}
		keys = append(keys, obj.Key)
	}
	return keys, nil
}

// Bucket returns the bucket name.
func (s *Store) Bucket() string {
	return s.bucket
}

// Client returns the underlying minio client (used for backup).
func (s *Store) Client() *minio.Client {
	return s.client
}
