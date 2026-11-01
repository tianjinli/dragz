package jobs

import (
	"context"
	"fmt"
	"time"

	"dragz-admin/backend/internal/filestore"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

type secretDecryptor interface {
	Decrypt(value string) (string, error)
}

func cleanupStorageTestFiles(ctx context.Context, db *store.DB, repo *store.AuthRepository, secrets secretDecryptor, retentionHours int) (string, error) {
	if repo == nil {
		return "", fmt.Errorf("storage cleanup requires repository")
	}
	configs, err := repo.ListStorageConfigs(ctx)
	if err != nil {
		return "", err
	}
	before := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	total := 0
	for _, config := range configs {
		if config.Status != 1 {
			continue
		}
		full, err := repo.StorageConfigByID(ctx, config.ID)
		if err != nil {
			return "", err
		}
		provider, err := storageProviderFromConfig(full, secrets)
		if err != nil {
			continue
		}
		count, err := provider.DeleteExpiredUnderPrefix(ctx, filestore.StorageTestPrefix, before)
		if err != nil {
			return "", fmt.Errorf("storage %d cleanup failed: %w", config.ID, err)
		}
		total += count
	}
	// Purge the file-management rows so expired tests disappear from the list.
	// Object deletion above lists storage directly by prefix, so rows and
	// objects converge even if one side lags a run behind.
	var purged int64
	if db != nil {
		res, err := db.ExecContext(ctx, `UPDATE sys_file SET deleted_at = NOW(), status = 'purged'
			WHERE deleted_at IS NULL AND status IN ('ready', 'failed')
			  AND object_key LIKE '_storage-test/%' AND created_at < $1`, before)
		if err != nil {
			return "", fmt.Errorf("purge storage test file records failed: %w", err)
		}
		purged, _ = res.RowsAffected()
	}
	return fmt.Sprintf("deleted %d storage test files, purged %d file records", total, purged), nil
}

func storageProviderFromConfig(config store.StorageConfig, secrets secretDecryptor) (filestore.Provider, error) {
	accessKey, secretKey := config.AccessKey, config.SecretKey
	if secrets != nil {
		var err error
		if accessKey != "" {
			accessKey, err = secrets.Decrypt(accessKey)
			if err != nil {
				return nil, err
			}
		}
		if secretKey != "" {
			secretKey, err = secrets.Decrypt(secretKey)
			if err != nil {
				return nil, err
			}
		}
	}
	return filestore.New(filestore.Config{
		Type: config.StorageType, Endpoint: config.Endpoint, Bucket: config.Bucket,
		Region: config.Region, AccessKey: accessKey, SecretKey: secretKey,
		BasePath: config.BasePath, UseHTTPS: config.UseHTTPS,
	})
}

var _ secretDecryptor = (*security.SecretBox)(nil)
