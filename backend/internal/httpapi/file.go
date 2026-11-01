package httpapi

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/filestore"
	"dragz-admin/backend/internal/store"
)

type saveFileReferenceRequest struct {
	BusinessType string `json:"businessType"`
	BusinessID   string `json:"businessId"`
	FieldName    string `json:"fieldName"`
}

func listFilesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListFiles(r.Context(), fileListFilterFromQuery(query, page, pageSize))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

// fileListFilterFromQuery maps supported list filters onto store.FileListFilter.
// The `recycled` boolean is a convenience alias used by the admin recycle bin:
// recycled=true lists only trashed files, recycled=false excludes them.
func fileListFilterFromQuery(query url.Values, page, pageSize int) store.FileListFilter {
	filter := store.FileListFilter{
		Keyword: query.Get("keyword"), MimeType: query.Get("mimeType"),
		Status: query.Get("status"), UploaderID: queryUint64(query.Get("uploaderId")),
		Page: page, PageSize: pageSize,
	}
	if raw := strings.TrimSpace(query.Get("recycled")); raw != "" {
		if recycled, err := strconv.ParseBool(raw); err == nil {
			if recycled {
				filter.Status = "recycled"
			} else {
				filter.ExcludeStatus = "recycled"
			}
		}
	}
	return filter
}

func getFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		item, err := ctx.Auth.FileByID(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.file_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_detail_failed")
			return
		}
		references, err := ctx.Auth.ListFileReferences(r.Context(), id)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_ref_list_failed")
			return
		}
		ok(w, r, map[string]any{"file": item, "references": references})
	}
}

func uploadFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024*1024)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_upload_form_invalid")
			return
		}
		storageID := queryUint64(r.FormValue("storageId"))
		config, err := loadStorageConfig(ctx, r, storageID)
		if err != nil {
			fail(w, r, http.StatusBadRequest, 40002, "errors.storage_unavailable")
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			fail(w, r, http.StatusBadRequest, 40003, "errors.file_required")
			return
		}
		defer file.Close()
		if header.Size <= 0 || uint64(header.Size) > config.MaxFileSize {
			fail(w, r, http.StatusBadRequest, 40004, "errors.file_too_large")
			return
		}
		extension := strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), "."))
		if !extensionAllowed(extension, config.AllowedExtensions) {
			fail(w, r, http.StatusBadRequest, 40005, "errors.file_ext_not_allowed")
			return
		}
		contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		accessType := strings.TrimSpace(r.FormValue("accessType"))
		if accessType == "" {
			if config.PrivateAccess {
				accessType = "private"
			} else {
				accessType = "public"
			}
		}
		if accessType != "private" && accessType != "public" {
			fail(w, r, http.StatusBadRequest, 40006, "errors.file_access_invalid")
			return
		}
		objectKey := time.Now().Format("2006/01/02") + "/" + uuid.NewString()
		if extension != "" {
			objectKey += "." + extension
		}
		uploaderID, _ := currentUserID(r.Context())
		fileID, err := ctx.Auth.CreatePendingFile(r.Context(), store.File{
			StorageID: config.ID, OriginalName: filepath.Base(header.Filename), ObjectKey: objectKey,
			Extension: extension, MimeType: contentType, Size: uint64(header.Size),
			AccessType: accessType, UploaderID: uploaderID,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_meta_create_failed")
			return
		}
		provider, err := storageProvider(ctx, config)
		if err != nil {
			ctx.Auth.FailFile(r.Context(), fileID, err.Error())
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_driver_failed")
			return
		}
		hash := sha256.New()
		if err := provider.Put(r.Context(), objectKey, io.TeeReader(file, hash), header.Size, contentType); err != nil {
			ctx.Auth.FailFile(r.Context(), fileID, err.Error())
			fail(w, r, http.StatusBadGateway, 50201, "errors.file_write_failed")
			return
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if err := ctx.Auth.CompleteFile(r.Context(), fileID, checksum); err != nil {
			_ = provider.Delete(r.Context(), objectKey)
			ctx.Auth.FailFile(r.Context(), fileID, err.Error())
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_confirm_failed")
			return
		}
		item, _ := ctx.Auth.FileByID(r.Context(), fileID)
		payload := map[string]any{
			"id": item.ID, "storageId": item.StorageID, "storageName": item.StorageName,
			"originalName": item.OriginalName, "objectKey": item.ObjectKey, "extension": item.Extension,
			"mimeType": item.MimeType, "size": item.Size, "sha256": item.SHA256,
			"accessType": item.AccessType, "status": item.Status, "uploaderId": item.UploaderID,
			"uploaderName": item.UploaderName, "uploadedAt": item.UploadedAt, "createdAt": item.CreatedAt,
		}
		if item.AccessType == "public" {
			payload["url"] = systemStoragePublicURL(config, item.ObjectKey, r, fileID)
		}
		ok(w, r, payload)
	}
}

func downloadFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		item, err := ctx.Auth.FileByID(r.Context(), id)
		if err != nil || item.Status != "ready" {
			fail(w, r, http.StatusNotFound, 40401, "errors.file_not_downloadable")
			return
		}
		writeFileContent(w, r, ctx, item, true)
	}
}

// publicFileContentHandler serves ready public files for <img src> / CDN-style access.
func publicFileContentHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		item, err := ctx.Auth.FileByID(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.file_not_found")
			return
		}
		if err != nil || item.Status != "ready" || item.AccessType != "public" {
			fail(w, r, http.StatusNotFound, 40401, "errors.file_not_found")
			return
		}
		writeFileContent(w, r, ctx, item, false)
	}
}

// systemPublicFileURL returns an absolute http(s) public content URL for echo/preview.
func systemPublicFileURL(r *http.Request, fileID uint64) string {
	scheme := "http"
	if r != nil && (r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")) {
		scheme = "https"
	}
	host := "localhost"
	if r != nil {
		if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
			host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
		} else if h := strings.TrimSpace(r.Host); h != "" {
			host = h
		}
	}
	return fmt.Sprintf("%s://%s/api/system/files/%d/content", scheme, host, fileID)
}

// storagePublicURLString returns the CDN public URL when publicBaseURL is configured,
// e.g. https://img.diy.cszqq.top/<objectKey>. Empty string means not configured.
func storagePublicURLString(publicBaseURL, objectKey string) string {
	base := strings.TrimSpace(publicBaseURL)
	if base == "" {
		return ""
	}
	lower := strings.ToLower(base)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return ""
	}
	key := strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if key == "" {
		return ""
	}
	return strings.TrimRight(base, "/") + "/" + key
}

// systemStoragePublicURL prefers the storage CDN address and falls back to the proxy URL.
func systemStoragePublicURL(config store.StorageConfig, objectKey string, r *http.Request, fileID uint64) string {
	if publicURL := storagePublicURLString(config.PublicBaseURL, objectKey); publicURL != "" {
		return publicURL
	}
	return systemPublicFileURL(r, fileID)
}

func writeFileContent(w http.ResponseWriter, r *http.Request, ctx *app.Context, item store.File, asAttachment bool) {
	config, err := ctx.Auth.StorageConfigByID(r.Context(), item.StorageID)
	if err != nil {
		fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_config_load_failed")
		return
	}
	if !asAttachment {
		if redirect := storagePublicURLString(config.PublicBaseURL, item.ObjectKey); redirect != "" {
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
	}
	provider, err := storageProvider(ctx, config)
	if err != nil {
		fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_driver_failed")
		return
	}
	reader, err := provider.Open(r.Context(), item.ObjectKey)
	if err != nil {
		fail(w, r, http.StatusBadGateway, 50201, "errors.storage_read_failed")
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", item.MimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", item.Size))
	if asAttachment {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.OriginalName}))
	} else {
		w.Header().Set("Cache-Control", "public, max-age=86400")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, reader)
}

func recycleFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		if err := ctx.Auth.RecycleFile(r.Context(), id, 30); err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.file_trash_only_ready")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func restoreFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		if err := ctx.Auth.RestoreFile(r.Context(), id); err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.file_restore_only_trash")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func purgeFileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		item, err := ctx.Auth.FileByID(r.Context(), id)
		if err != nil || (item.Status != "recycled" && item.Status != "failed") {
			fail(w, r, http.StatusConflict, 40901, "errors.file_purge_invalid")
			return
		}
		config, err := ctx.Auth.StorageConfigByID(r.Context(), item.StorageID)
		if err == nil && item.Status != "failed" {
			provider, providerErr := storageProvider(ctx, config)
			if providerErr != nil || provider.Delete(r.Context(), item.ObjectKey) != nil {
				fail(w, r, http.StatusBadGateway, 50201, "errors.storage_delete_failed")
				return
			}
		}
		if err := ctx.Auth.PurgeFileRecord(r.Context(), id); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.file_meta_purge_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func addFileReferenceHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_id_invalid")
			return
		}
		var req saveFileReferenceRequest
		if !decode(w, r, &req) {
			return
		}
		req.BusinessType, req.BusinessID, req.FieldName = strings.TrimSpace(req.BusinessType), strings.TrimSpace(req.BusinessID), strings.TrimSpace(req.FieldName)
		if req.BusinessType == "" || req.BusinessID == "" || len(req.BusinessType) > 64 || len(req.BusinessID) > 128 {
			fail(w, r, http.StatusBadRequest, 40002, "errors.file_ref_biz_required")
			return
		}
		if req.FieldName == "" {
			req.FieldName = "attachments"
		}
		userID, _ := currentUserID(r.Context())
		referenceID, err := ctx.Auth.AddFileReference(r.Context(), store.FileReference{
			FileID: id, BusinessType: req.BusinessType, BusinessID: req.BusinessID,
			FieldName: req.FieldName, CreatedBy: userID,
		})
		if err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.file_ref_conflict")
			return
		}
		ok(w, r, map[string]uint64{"id": referenceID})
	}
}

func deleteFileReferenceHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.file_ref_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteFileReference(r.Context(), id); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.file_ref_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func testStorageConfigHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.storage_id_invalid")
			return
		}
		config, err := ctx.Auth.StorageConfigByID(r.Context(), id)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.storage_not_found")
			return
		}
		provider, err := storageProvider(ctx, config)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50201, "errors.storage_test_failed")
			return
		}
		size := int64(len(storageTestFaviconSVG))
		// Unique key per run so every connectivity test shows up as its own
		// row in file management and expires independently. The
		// _storage-test/ prefix is reaped by maintenance.cleanup_storage_test_files.
		objectKey := filestore.StorageTestPrefix + time.Now().Format("2006/01/02") +
			"/test-" + uuid.NewString() + ".svg"
		uploaderID, _ := currentUserID(r.Context())
		fileID, err := ctx.Auth.CreatePendingFile(r.Context(), store.File{
			StorageID: config.ID, OriginalName: "storage-connection-test.svg", ObjectKey: objectKey,
			Extension: "svg", MimeType: "image/svg+xml", Size: uint64(size),
			AccessType: "public", UploaderID: uploaderID,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_test_failed")
			return
		}
		if err := provider.Put(r.Context(), objectKey, bytes.NewReader(storageTestFaviconSVG), size, "image/svg+xml"); err != nil {
			ctx.Auth.FailFile(r.Context(), fileID, err.Error())
			fail(w, r, http.StatusBadGateway, 50201, "errors.storage_test_failed")
			return
		}
		sum := sha256.Sum256(storageTestFaviconSVG)
		if err := ctx.Auth.CompleteFile(r.Context(), fileID, hex.EncodeToString(sum[:])); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_test_failed")
			return
		}
		ok(w, r, map[string]any{
			"success": true, "objectKey": objectKey,
			"fileId": fileID, "url": systemStoragePublicURL(config, objectKey, r, fileID),
		})
	}
}

func loadStorageConfig(ctx *app.Context, r *http.Request, storageID uint64) (store.StorageConfig, error) {
	var config store.StorageConfig
	var err error
	if storageID > 0 {
		config, err = ctx.Auth.StorageConfigByID(r.Context(), storageID)
	} else {
		config, err = ctx.Auth.DefaultStorageConfig(r.Context())
	}
	if err != nil || config.Status != 1 {
		return store.StorageConfig{}, errors.New("storage config is unavailable")
	}
	return config, nil
}

func storageProvider(ctx *app.Context, config store.StorageConfig) (filestore.Provider, error) {
	accessKey, err := ctx.Secrets.Decrypt(config.AccessKey)
	if err != nil {
		return nil, err
	}
	secretKey, err := ctx.Secrets.Decrypt(config.SecretKey)
	if err != nil {
		return nil, err
	}
	return filestore.New(filestore.Config{
		Type: config.StorageType, Endpoint: config.Endpoint, Bucket: config.Bucket,
		Region: config.Region, AccessKey: accessKey, SecretKey: secretKey,
		BasePath: config.BasePath, UseHTTPS: config.UseHTTPS,
	})
}

func extensionAllowed(extension, allowed string) bool {
	if strings.TrimSpace(allowed) == "" {
		return true
	}
	for _, value := range strings.Split(allowed, ",") {
		if strings.EqualFold(strings.Trim(strings.TrimSpace(value), "."), extension) {
			return true
		}
	}
	return false
}
