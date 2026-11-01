package store

import (
	"context"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// File is one sys_file row.
type File struct {
	ID             uint64     `json:"id"`
	StorageID      uint64     `json:"storageId"`
	StorageName    string     `json:"storageName"`
	OriginalName   string     `json:"originalName"`
	ObjectKey      string     `json:"objectKey"`
	Extension      string     `json:"extension"`
	MimeType       string     `json:"mimeType"`
	Size           uint64     `json:"size"`
	SHA256         string     `json:"sha256"`
	AccessType     string     `json:"accessType"`
	Status         string     `json:"status"`
	UploaderID     uint64     `json:"uploaderId"`
	UploaderName   string     `json:"uploaderName"`
	UploadedAt     *time.Time `json:"uploadedAt"`
	RecycleAt      *time.Time `json:"recycleAt"`
	PurgeAfter     *time.Time `json:"purgeAfter"`
	ReferenceCount int64      `json:"referenceCount"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// FileReference records that a business row points at a file, which is what protects the file from being purged.
type FileReference struct {
	ID           uint64    `json:"id"`
	FileID       uint64    `json:"fileId"`
	BusinessType string    `json:"businessType"`
	BusinessID   string    `json:"businessId"`
	FieldName    string    `json:"fieldName"`
	CreatedBy    uint64    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FileListFilter is the query string of the file list. ExcludeStatus is how the recycle bin and the live list are separated.
type FileListFilter struct {
	Keyword       string
	MimeType      string
	Status        string
	ExcludeStatus string
	UploaderID    uint64
	Page          int
	PageSize      int
}

// CreatePendingFile records the metadata before the bytes are written, so a failed upload leaves a row that can be cleaned up.
func (r *AuthRepository) CreatePendingFile(ctx context.Context, file File) (uint64, error) {
	id, err := r.queries.CreatePendingFile(ctx, sqlcstore.CreatePendingFileParams{
		StorageID: file.StorageID, OriginalName: file.OriginalName, ObjectKey: file.ObjectKey,
		Extension: file.Extension, MimeType: file.MimeType, Size: file.Size,
		AccessType: file.AccessType, UploaderID: file.UploaderID,
	})
	return uint64(id), err
}

// CompleteFile marks a pending upload as ready and stores its checksum.
func (r *AuthRepository) CompleteFile(ctx context.Context, fileID uint64, sha256 string) error {
	affected, err := r.queries.CompleteFile(ctx, sqlcstore.CompleteFileParams{Sha256: sha256, ID: int64(fileID)})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrFileStateConflict
	}
	return nil
}

// FailFile marks a pending upload as failed. The error is intentionally swallowed: the caller is already handling a failure.
func (r *AuthRepository) FailFile(ctx context.Context, fileID uint64, reason string) {
	_ = r.queries.FailFile(ctx, sqlcstore.FailFileParams{PurgeError: reason, ID: int64(fileID)})
}

// ListFiles returns one page of files plus the total count.
func (r *AuthRepository) ListFiles(ctx context.Context, filter FileListFilter) ([]File, int64, error) {
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	params := sqlcstore.CountFilesFilteredParams{
		KeywordPattern: optionalPattern(filter.Keyword),
		MimePattern:    optionalPrefix(filter.MimeType),
		Status:         optionalString(filter.Status),
		ExcludeStatus:  optionalString(filter.ExcludeStatus),
		UploaderID:     optionalUint64Null(filter.UploaderID),
	}
	total, err := r.queries.CountFilesFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListFilesFiltered(ctx, sqlcstore.ListFilesFilteredParams{
		Limit: int32(filter.PageSize), Offset: int32((filter.Page - 1) * filter.PageSize),
		KeywordPattern: params.KeywordPattern, MimePattern: params.MimePattern,
		Status: params.Status, ExcludeStatus: params.ExcludeStatus, UploaderID: params.UploaderID,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]File, 0, len(rows))
	for _, row := range rows {
		items = append(items, File{
			ID: uint64(row.ID), StorageID: row.StorageID, StorageName: row.StorageName,
			OriginalName: row.OriginalName, ObjectKey: row.ObjectKey, Extension: row.Extension,
			MimeType: row.MimeType, Size: row.Size, SHA256: row.Sha256, AccessType: row.AccessType,
			Status: row.Status, UploaderID: row.UploaderID, UploaderName: row.UploaderName,
			UploadedAt: nullTimePtr(row.UploadedAt), RecycleAt: nullTimePtr(row.RecycleAt),
			PurgeAfter: nullTimePtr(row.PurgeAfter), ReferenceCount: row.ReferenceCount, CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// FileByID loads one file by primary key.
func (r *AuthRepository) FileByID(ctx context.Context, fileID uint64) (File, error) {
	row, err := r.queries.GetFileByID(ctx, int64(fileID))
	if err != nil {
		return File{}, err
	}
	return File{
		ID: uint64(row.ID), StorageID: row.StorageID, StorageName: row.StorageName,
		OriginalName: row.OriginalName, ObjectKey: row.ObjectKey, Extension: row.Extension,
		MimeType: row.MimeType, Size: row.Size, SHA256: row.Sha256, AccessType: row.AccessType,
		Status: row.Status, UploaderID: row.UploaderID, UploaderName: row.UploaderName,
		UploadedAt: nullTimePtr(row.UploadedAt), RecycleAt: nullTimePtr(row.RecycleAt),
		PurgeAfter: nullTimePtr(row.PurgeAfter), ReferenceCount: row.ReferenceCount, CreatedAt: row.CreatedAt,
	}, nil
}

// RecycleFile moves a ready file to the recycle bin and schedules its purge.
func (r *AuthRepository) RecycleFile(ctx context.Context, fileID uint64, days int) error {
	affected, err := r.queries.RecycleFile(ctx, sqlcstore.RecycleFileParams{Days: int32(days), ID: int64(fileID)})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrFileStateConflict
	}
	return nil
}

// RestoreFile moves a recycled file back to ready and clears the purge deadline.
func (r *AuthRepository) RestoreFile(ctx context.Context, fileID uint64) error {
	affected, err := r.queries.RestoreFile(ctx, int64(fileID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrFileStateConflict
	}
	return nil
}

// PurgeFileRecord deletes the reference rows and then the file row in one transaction.
func (r *AuthRepository) PurgeFileRecord(ctx context.Context, fileID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := q.DeleteFileReferences(ctx, fileID); err != nil {
		return err
	}
	affected, err := q.PurgeFileRecord(ctx, int64(fileID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrFileStateConflict
	}
	return tx.Commit()
}

// AddFileReference links a business row to a file.
func (r *AuthRepository) AddFileReference(ctx context.Context, reference FileReference) (uint64, error) {
	id, err := r.queries.AddFileReference(ctx, sqlcstore.AddFileReferenceParams{
		FileID: reference.FileID, BusinessType: reference.BusinessType, BusinessID: reference.BusinessID,
		FieldName: reference.FieldName, CreatedBy: reference.CreatedBy,
	})
	return uint64(id), err
}

// DeleteFileReference removes one link, which makes the file eligible for purging again.
func (r *AuthRepository) DeleteFileReference(ctx context.Context, referenceID uint64) error {
	affected, err := r.queries.DeleteFileReference(ctx, int64(referenceID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ListFileReferences returns every business row that points at a file.
func (r *AuthRepository) ListFileReferences(ctx context.Context, fileID uint64) ([]FileReference, error) {
	rows, err := r.queries.ListFileReferences(ctx, fileID)
	if err != nil {
		return nil, err
	}
	items := make([]FileReference, 0, len(rows))
	for _, row := range rows {
		items = append(items, FileReference{
			ID: uint64(row.ID), FileID: row.FileID, BusinessType: row.BusinessType,
			BusinessID: row.BusinessID, FieldName: row.FieldName, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}
