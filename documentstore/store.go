// Package documentstore provides backend-neutral immutable storage for raw
// uploads.  Callers deliberately depend on object semantics, never paths.
package documentstore

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-kit/log"
	"github.com/thanos-io/objstore"
	"github.com/thanos-io/objstore/providers/filesystem"
	"github.com/thanos-io/objstore/providers/s3"
)

const (
	BackendFilesystem = "filesystem"
	BackendS3         = "s3"
)

// Options is constructed and validated by the e-ubl21 runtime configuration
// layer. This package intentionally does not read environment variables or
// config files. S3 fields apply equally to AWS S3 and compatible endpoints.
type Options struct {
	Backend      string
	Immutability string
	Directory    string
	Bucket       string
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	UseTLS       bool
}

const (
	ImmutabilityNativeConditional = "native-conditional"
	ImmutabilityGuardedWrite      = "guarded-write"
)

// Store is the narrow application contract for immutable upload objects.
type Store interface {
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
	Size(context.Context) (uint64, error)
	IsNotFound(error) bool
}

// New opens the configured object bucket.
func New(cfg Options) (Store, error) {
	if cfg.Immutability == "" {
		cfg.Immutability = ImmutabilityNativeConditional
	}
	if cfg.Immutability != ImmutabilityNativeConditional && cfg.Immutability != ImmutabilityGuardedWrite {
		return nil, fmt.Errorf("unsupported object-store immutability mode %q", cfg.Immutability)
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Backend)) {
	case BackendFilesystem:
		if strings.TrimSpace(cfg.Directory) == "" {
			return nil, fmt.Errorf("object-store filesystem directory is required")
		}
		bucket, err := filesystem.NewBucket(cfg.Directory)
		if err != nil {
			return nil, fmt.Errorf("open filesystem object store: %w", err)
		}
		return &store{bucket: bucket, immutability: cfg.Immutability}, nil
	case BackendS3:
		if strings.TrimSpace(cfg.Bucket) == "" {
			return nil, fmt.Errorf("object-store S3 bucket is required")
		}
		bucket, err := s3.NewBucketWithConfig(log.NewNopLogger(), s3.Config{Bucket: cfg.Bucket, Endpoint: cfg.Endpoint, Region: cfg.Region, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey, Insecure: !cfg.UseTLS, DisableMultipart: true}, "e-ubl21", nil)
		if err != nil {
			return nil, fmt.Errorf("open S3 object store: %w", err)
		}
		return &store{bucket: bucket, immutability: cfg.Immutability}, nil
	default:
		return nil, fmt.Errorf("unsupported object-store backend %q", cfg.Backend)
	}
}

type store struct {
	bucket       objstore.Bucket
	immutability string
}

func ObjectKey(uploadID string) string { return "uploads/" + uploadID + "/document.xml" }

func (s *store) Put(ctx context.Context, key string, content io.Reader) error {
	var err error
	if s.immutability == ImmutabilityGuardedWrite {
		err = s.bucket.Upload(ctx, key, content)
	} else {
		err = s.bucket.Upload(ctx, key, content, objstore.WithIfNotExists())
	}
	if err != nil {
		return err
	}
	return nil
}
func (s *store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.bucket.Get(ctx, key)
}
func (s *store) Delete(ctx context.Context, key string) error { return s.bucket.Delete(ctx, key) }
func (s *store) Exists(ctx context.Context, key string) (bool, error) {
	return s.bucket.Exists(ctx, key)
}
func (s *store) Size(ctx context.Context) (uint64, error) {
	var size uint64
	err := s.bucket.Iter(ctx, "", func(name string) error {
		attrs, err := s.bucket.Attributes(ctx, name)
		if err != nil {
			return err
		}
		if attrs.Size > 0 {
			size += uint64(attrs.Size)
		}
		return nil
	}, objstore.WithRecursiveIter())
	return size, err
}
func (s *store) IsNotFound(err error) bool { return s.bucket.IsObjNotFoundErr(err) }
