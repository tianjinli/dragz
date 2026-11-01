package jobs

import (
	"context"
	"fmt"
	"strings"

	"dragz-admin/backend/internal/filestore"
	"dragz-admin/backend/internal/store"
)

// 孤儿文件 = 上传成功落库（sys_file.status='ready'）但已没有任何业务记录指向它的对象。
// 上传后放弃提交、换图、删作品/清草稿都会留下这种文件，只能定时回收。
//
// 引用来源分两类，必须同时看：
//   - 文本引用：业务表里以字符串保存的地址，例如 /api/system/files/{id}/content、
//     CDN 直链（https://<public_base_url>/<object_key>），以及嵌在 JSONB 里的同类地址；
//   - 裸 id 引用：sys_file_reference.file_id、sys_user.avatar_file_id 等。
//
// 删除分两步且都可回退：先把孤儿标记为 recycled 进回收站（后台可还原），
// 只有在回收站里过了 purge_after 才真正删对象并置 purged。

// fileRefTextSource 描述一张可能以文本形式保存文件地址的业务表。
type fileRefTextSource struct {
	table string
	// expr 从该表取出可能内嵌 object_key 或 /files/{id}/content 的值表达式。
	expr string
}

// fileRefIDSource 描述一张以裸文件 id 引用 sys_file 的业务表。
type fileRefIDSource struct {
	table  string
	column string
}

// orphanFileTextSources 枚举所有以文本形式存文件地址的位置。
// 新增会保存图片/视频地址的字段时必须同步这里，否则在用文件会被当成孤儿回收。
// 表不存在时会被自动跳过，因此删表后不需要回头改这段。
var orphanFileTextSources = []fileRefTextSource{}

// orphanFileIDSources 枚举所有以裸 id 存文件引用的位置。
var orphanFileIDSources = []fileRefIDSource{
	{table: "sys_file_reference", column: "file_id"},
	{table: "sys_user", column: "avatar_file_id"},
}

// buildRecycleOrphanFilesSQL 按实际存在的表拼出回收 SQL，避免引用已删除的业务表导致整条语句失败。
func buildRecycleOrphanFilesSQL(textSources []fileRefTextSource, idSources []fileRefIDSource) string {
	refLines := make([]string, 0, len(textSources))
	for _, src := range textSources {
		refLines = append(refLines, "SELECT NULLIF(BTRIM("+src.expr+"), '') AS value FROM "+src.table)
	}
	refsBody := "SELECT NULL::text AS value WHERE FALSE"
	if len(refLines) > 0 {
		refsBody = strings.Join(refLines, "\n\t\tUNION ALL ")
	}

	bareLines := make([]string, 0, len(idSources))
	for _, src := range idSources {
		bareLines = append(bareLines, "SELECT "+src.column+" AS file_id FROM "+src.table+" WHERE "+src.column+" IS NOT NULL")
	}
	bareBody := "SELECT NULL::bigint AS file_id WHERE FALSE"
	if len(bareLines) > 0 {
		bareBody = strings.Join(bareLines, "\n\t\tUNION ")
	}

	return `WITH refs(value) AS (
		` + refsBody + `
	),
	ref_ids AS (
		SELECT DISTINCT (matched)[1]::BIGINT AS file_id
		FROM refs, REGEXP_MATCHES(refs.value, '/files/([0-9]+)/content', 'g') AS matched
		WHERE refs.value IS NOT NULL
	),
	bare_ids AS (
		` + bareBody + `
	),
	candidates AS (
		SELECT f.id
		FROM sys_file f
		WHERE f.status = 'ready' AND f.deleted_at IS NULL
		  AND f.object_key <> ''
		  AND f.object_key NOT LIKE '` + filestore.StorageTestPrefix + `%'
		  AND f.created_at < NOW() - ($1::int * INTERVAL '1 day')

		  AND NOT EXISTS (SELECT 1 FROM ref_ids r WHERE r.file_id = f.id)
		  AND NOT EXISTS (SELECT 1 FROM bare_ids b WHERE b.file_id = f.id)
		  AND NOT EXISTS (SELECT 1 FROM refs v
		                   WHERE v.value IS NOT NULL AND POSITION(f.object_key IN v.value) > 0)
		ORDER BY f.id
		LIMIT $2::int
	)
	UPDATE sys_file f
	SET status = 'recycled', recycle_at = NOW(), purge_after = NOW() + ($3::int * INTERVAL '1 day')
	WHERE f.id IN (SELECT id FROM candidates)`
}

// filterExistingSources 只保留当前库里真实存在的表。
func filterExistingSources(ctx context.Context, db *store.DB, textSources []fileRefTextSource, idSources []fileRefIDSource) ([]fileRefTextSource, []fileRefIDSource, error) {
	names := make([]string, 0, len(textSources)+len(idSources))
	for _, src := range textSources {
		names = append(names, src.table)
	}
	for _, src := range idSources {
		names = append(names, src.table)
	}
	if len(names) == 0 {
		return nil, nil, nil
	}
	rows, err := db.QueryContext(ctx, existingTablesSQL, strings.Join(names, ","))
	if err != nil {
		return nil, nil, fmt.Errorf("list file reference tables failed: %w", err)
	}
	defer func() { _ = rows.Close() }()
	present := make(map[string]bool, len(names))
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, nil, fmt.Errorf("read file reference table failed: %w", scanErr)
		}
		present[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	keptText := make([]fileRefTextSource, 0, len(textSources))
	for _, src := range textSources {
		if present[src.table] {
			keptText = append(keptText, src)
		}
	}
	keptID := make([]fileRefIDSource, 0, len(idSources))
	for _, src := range idSources {
		if present[src.table] {
			keptID = append(keptID, src)
		}
	}
	return keptText, keptID, nil
}

const existingTablesSQL = `SELECT table_name FROM information_schema.tables
	WHERE table_schema = current_schema() AND table_name = ANY(string_to_array($1, ','))`

const duePurgedFilesSQL = `
	SELECT id, storage_id, object_key
	FROM sys_file
	WHERE status = 'recycled' AND deleted_at IS NULL
	  AND purge_after IS NOT NULL AND purge_after < NOW()
	ORDER BY purge_after
	LIMIT $1::int`

const purgeFileRecordSQL = `UPDATE sys_file SET status = 'purged', deleted_at = NOW()
	WHERE id = $1 AND status = 'recycled' AND deleted_at IS NULL`

const deleteFileReferencesSQL = `DELETE FROM sys_file_reference WHERE file_id = $1`

type orphanFile struct {
	id        uint64
	storageID uint64
	objectKey string
}

// cleanupOrphanFiles 先回收无引用的新文件，再删掉回收站里已到期的对象。
// 单个对象删除失败只跳过该条，留给下一轮，不影响其余文件。
func cleanupOrphanFiles(ctx context.Context, db *store.DB, repo *store.AuthRepository, secrets secretDecryptor, graceDays, maxFiles, recycleDays int) (string, error) {
	if db == nil {
		return "", fmt.Errorf("orphan file cleanup requires database")
	}
	if graceDays < 1 {
		graceDays = 7
	}
	if maxFiles < 1 {
		maxFiles = 1000
	}
	if recycleDays < 1 {
		recycleDays = 30
	}
	textSources, idSources, err := filterExistingSources(ctx, db, orphanFileTextSources, orphanFileIDSources)
	if err != nil {
		return "", err
	}
	recycleSQL := buildRecycleOrphanFilesSQL(textSources, idSources)
	res, err := db.ExecContext(ctx, recycleSQL, graceDays, maxFiles, recycleDays)
	if err != nil {
		return "", fmt.Errorf("recycle orphan files failed: %w", err)
	}
	recycled, _ := res.RowsAffected()
	purged, skipped, err := purgeExpiredFiles(ctx, db, repo, secrets, maxFiles)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("recycled %d orphan files, purged %d, kept %d undeletable", recycled, purged, skipped), nil
}

func purgeExpiredFiles(ctx context.Context, db *store.DB, repo *store.AuthRepository, secrets secretDecryptor, limit int) (purged, skipped int, err error) {
	rows, err := db.QueryContext(ctx, duePurgedFilesSQL, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("list expired recycled files failed: %w", err)
	}
	due := make([]orphanFile, 0, limit)
	for rows.Next() {
		var item orphanFile
		if scanErr := rows.Scan(&item.id, &item.storageID, &item.objectKey); scanErr != nil {
			_ = rows.Close()
			return 0, 0, fmt.Errorf("read expired recycled file failed: %w", scanErr)
		}
		due = append(due, item)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return 0, 0, closeErr
	}
	if len(due) == 0 {
		return 0, 0, nil
	}
	// 同一次任务里反复读同一份存储配置，避免每个文件都查一次。
	providers := make(map[uint64]filestore.Provider, 2)
	for _, item := range due {
		provider, ok := providers[item.storageID]
		if !ok {
			if repo == nil {
				skipped++
				continue
			}
			config, configErr := repo.StorageConfigByID(ctx, item.storageID)
			if configErr != nil {
				skipped++
				continue
			}
			provider, configErr = storageProviderFromConfig(config, secrets)
			if configErr != nil {
				skipped++
				continue
			}
			providers[item.storageID] = provider
		}
		if delErr := provider.Delete(ctx, item.objectKey); delErr != nil {
			skipped++
			continue
		}
		if err := purgeFileRow(ctx, db, item.id); err != nil {
			return purged, skipped, err
		}
		purged++
	}
	return purged, skipped, nil
}

func purgeFileRow(ctx context.Context, db *store.DB, fileID uint64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, deleteFileReferencesSQL, fileID); err != nil {
		return fmt.Errorf("drop references of file %d failed: %w", fileID, err)
	}
	res, err := tx.ExecContext(ctx, purgeFileRecordSQL, fileID)
	if err != nil {
		return fmt.Errorf("purge file %d failed: %w", fileID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("file %d is no longer recyclable", fileID)
	}
	return tx.Commit()
}
