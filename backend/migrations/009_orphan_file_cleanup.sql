BEGIN;
SELECT pg_advisory_xact_lock(928048);

-- 孤儿文件清理：上传成功但已无任何业务记录指向的文件（换图、放弃提交、删作品后留下）。
-- 先软回收进文件回收站（可还原），只有回收站里过了 purge_after 才真正删对象，
-- 因此首次部署后的 37 天内不会有任何物理删除。宽限期与保留期都在 payload 里调。
INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT '清理孤儿文件', 'maintenance.orphan_files', 'maintenance.cleanup_orphan_files',
       '0 4 * * *', '{"graceDays": 7, "recycleDays": 30, "maxFiles": 1000}'::jsonb, 300, 1, 1,
       NOW() + INTERVAL '1 day'
WHERE NOT EXISTS (SELECT 1 FROM sys_scheduled_job WHERE code = 'maintenance.orphan_files' AND deleted_at IS NULL);

COMMIT;
