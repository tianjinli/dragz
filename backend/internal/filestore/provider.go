// Package filestore abstracts object storage. The local filesystem and
// S3-compatible backends implement the same Provider contract, so the upload
// handlers never branch on the backend.
package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const StorageTestPrefix = "_storage-test/"

// Provider is the storage backend contract. Both the local filesystem and S3-compatible object storage implement it, which keeps the upload handlers backend-agnostic.
type Provider interface {
	Put(ctx context.Context, key string, source io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Test(ctx context.Context) error
	DeleteExpiredUnderPrefix(ctx context.Context, prefix string, before time.Time) (int, error)
}

// Config is the resolved settings of one backend. AccessKey and SecretKey arrive already decrypted.
type Config struct {
	Type      string
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	BasePath  string
	UseHTTPS  bool
}

// New builds a provider from a storage config. Local storage requires a base path; S3 requires a reachable endpoint and bucket.
func New(config Config) (Provider, error) {
	switch config.Type {
	case "local":
		if strings.TrimSpace(config.BasePath) == "" {
			return nil, errors.New("local base path is required")
		}
		root, err := filepath.Abs(config.BasePath)
		if err != nil {
			return nil, err
		}
		return &localProvider{root: root}, nil
	case "s3":
		endpoint := strings.TrimSpace(config.Endpoint)
		if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
			endpoint = parsed.Host
		}
		client, err := minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
			Secure: config.UseHTTPS,
			Region: config.Region,
			// Tencent COS (and virtual-hosted S3 in general) rejects
			// path-style requests; minio-go defaults to path-style for
			// non-AWS endpoints, so force DNS (virtual-hosted) style.
			BucketLookup: minio.BucketLookupDNS,
		})
		if err != nil {
			return nil, err
		}
		return &s3Provider{client: client, bucket: config.Bucket}, nil
	default:
		return nil, errors.New("unsupported storage type")
	}
}

type localProvider struct {
	root string
}

func (p *localProvider) resolve(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(key, "/")))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", errors.New("invalid object key")
	}
	path := filepath.Join(p.root, clean)
	relative, err := filepath.Rel(p.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("object key escapes storage root")
	}
	return path, nil
}

func (p *localProvider) Put(_ context.Context, key string, source io.Reader, _ int64, _ string) error {
	path, err := p.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()
	if _, err := io.Copy(temporary, source); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func (p *localProvider) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := p.resolve(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (p *localProvider) Delete(_ context.Context, key string) error {
	path, err := p.resolve(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (p *localProvider) Test(_ context.Context) error {
	if err := os.MkdirAll(p.root, 0o750); err != nil {
		return err
	}
	file, err := os.CreateTemp(p.root, ".storage-test-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func (p *localProvider) DeleteExpiredUnderPrefix(_ context.Context, prefix string, before time.Time) (int, error) {
	root, err := p.resolve(prefix)
	if err != nil {
		return 0, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	if !info.IsDir() {
		if info.ModTime().Before(before) {
			if err := os.Remove(root); err != nil && !errors.Is(err, os.ErrNotExist) {
				return 0, err
			}
			return 1, nil
		}
		return 0, nil
	}
	deleted := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		fileInfo, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if !fileInfo.ModTime().Before(before) {
			return nil
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		deleted++
		return nil
	})
	if err != nil {
		return deleted, err
	}
	_ = removeEmptyDirs(root)
	return deleted, nil
}

func removeEmptyDirs(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || !entry.IsDir() || path == root {
			return err
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		if len(entries) == 0 {
			return os.Remove(path)
		}
		return nil
	})
}

type s3Provider struct {
	client *minio.Client
	bucket string
}

func (p *s3Provider) Put(ctx context.Context, key string, source io.Reader, size int64, contentType string) error {
	_, err := p.client.PutObject(ctx, p.bucket, key, source, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (p *s3Provider) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := p.client.GetObject(ctx, p.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (p *s3Provider) Delete(ctx context.Context, key string) error {
	return p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{})
}

func (p *s3Provider) Test(ctx context.Context) error {
	// Prove write access with a throwaway object. BucketExists (HeadBucket)
	// needs bucket-level permission that data-only credentials typically
	// lack, while Put/Delete is exactly what the uploader requires.
	key := StorageTestPrefix + "connectivity.txt"
	body := []byte("ok")
	if err := p.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "text/plain"); err != nil {
		return err
	}
	_ = p.Delete(ctx, key)
	return nil
}

func (p *s3Provider) DeleteExpiredUnderPrefix(ctx context.Context, prefix string, before time.Time) (int, error) {
	deleted := 0
	for object := range p.client.ListObjects(ctx, p.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return deleted, object.Err
		}
		if !object.LastModified.Before(before) {
			continue
		}
		if err := p.client.RemoveObject(ctx, p.bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}
