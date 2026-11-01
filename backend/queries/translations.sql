-- name: ListTranslationsByKey :many
SELECT entity_key, value FROM sys_translation
WHERE lang = $1 AND entity_key = ANY($2::text[]);

-- name: UpsertTranslationIfAbsent :exec
-- Seed-time insert only: an administrator-edited translation (or one written
-- by a previous run) must never be clobbered by the compiled-in default.
INSERT INTO sys_translation (entity_key, lang, value)
VALUES ($1, $2, $3)
ON CONFLICT (entity_key, lang) DO NOTHING;

-- name: ListTranslationRows :many
-- Paged translation records with their Chinese source text, for the
-- translation management page. Only rows that exist are listed; entities
-- without a translation show up in ListUntranslatedEntities instead.
-- Every row is validated against v_preset_entity, the single catalog of
-- translatable preset entities (see schema/tables.sql).
SELECT t.id, t.entity_key::text AS entity_key, e.source::text AS source,
       t.value, t.lang, t.updated_at
FROM sys_translation t
JOIN v_preset_entity e ON e.entity_key = t.entity_key
WHERE (sqlc.arg(lang)::text = '' OR t.lang = sqlc.arg(lang))
  AND (
    sqlc.arg(keyword)::text = ''
    OR t.entity_key ILIKE sqlc.arg(keyword)::text
    OR e.source ILIKE sqlc.arg(keyword)::text
    OR t.value ILIKE sqlc.arg(keyword)::text
  )
ORDER BY t.entity_key, t.lang
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountTranslationRows :one
SELECT count(*)
FROM sys_translation t
JOIN v_preset_entity e ON e.entity_key = t.entity_key
WHERE (sqlc.arg(lang)::text = '' OR t.lang = sqlc.arg(lang))
  AND (
    sqlc.arg(keyword)::text = ''
    OR t.entity_key ILIKE sqlc.arg(keyword)::text
    OR e.source ILIKE sqlc.arg(keyword)::text
    OR t.value ILIKE sqlc.arg(keyword)::text
  );

-- name: ListUntranslatedEntities :many
-- Preset entities that have no translation in the given language; these are
-- the choices offered by the "new translation" form.
SELECT e.entity_key::text AS entity_key, e.source::text AS source
FROM v_preset_entity e
LEFT JOIN sys_translation t ON t.entity_key = e.entity_key AND t.lang = $1
WHERE t.id IS NULL
  AND ($2::text = '' OR e.entity_key ILIKE $2::text OR e.source ILIKE $2::text)
ORDER BY e.entity_key
LIMIT 200;

-- name: ListSourceEntities :many
-- The source-language side of the translation page: every preset entity with
-- the text its owning table holds, so Chinese is editable from the same page
-- instead of being read-only. The value column is the source text itself.
-- `id` is a display key only — a source edit is addressed by entity_key — so it
-- starts above anything sys_translation's sequence can reach, and an id that
-- leaks into a record delete hits no row at all. The window count lets one
-- query serve both the page and its total.
SELECT e.entity_key::text AS entity_key,
       e.source::text AS source,
       e.source::text AS value,
       'zh'::text AS lang,
       e.updated_at,
       (1000000000000000 + ROW_NUMBER() OVER (ORDER BY e.entity_key))::bigint AS id,
       COUNT(*) OVER ()::bigint AS total
FROM v_preset_entity e
WHERE (sqlc.arg(keyword)::text = ''
       OR e.entity_key ILIKE sqlc.arg(keyword)::text
       OR e.source ILIKE sqlc.arg(keyword)::text)
ORDER BY e.entity_key
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- ---- source-language write-back -------------------------------------------
-- The source language lives in the owning table, so editing it on the
-- translation page updates that column. One query per namespace is what
-- store.(*AuthRepository).writeSourceText dispatches to; adding a translatable
-- entity means a UNION in v_preset_entity plus one of these and one case there.

-- name: SetMenuSourceText :execrows
-- Covers both menu.<code> and perm.<code>: sys_menu.code is unique across
-- pages and buttons, so one key addresses exactly one row.
UPDATE sys_menu SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetRoleSourceText :execrows
UPDATE sys_role SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetRoleSourceRemark :execrows
UPDATE sys_role SET remark = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetDepartmentSourceText :execrows
UPDATE sys_department SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetPositionSourceText :execrows
UPDATE sys_position SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetPositionSourceRemark :execrows
UPDATE sys_position SET remark = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetJobSourceText :execrows
UPDATE sys_scheduled_job SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetStorageSourceText :execrows
-- storage_type carries no UNIQUE constraint, so the storage.<type> key
-- addresses the oldest live row of that type; a duplicate type keeps its own
-- name rather than being renamed behind the administrator's back.
UPDATE sys_storage_config SET name = sqlc.arg(value), updated_at = NOW()
WHERE id = (
  SELECT MIN(sc.id) FROM sys_storage_config sc
  WHERE sc.storage_type = sqlc.arg(code) AND sc.deleted_at IS NULL
);

-- name: RekeyTranslations :many
-- A translation record belongs to the entity, not to its code, so editing a
-- code carries the records over instead of orphaning them (an orphaned key is
-- invisible: every list joins v_preset_entity). Rows whose destination is
-- already taken are left alone rather than violating uk_translation_entity_lang.
UPDATE sys_translation SET entity_key = sqlc.arg(new_key), updated_at = NOW()
WHERE sys_translation.entity_key = sqlc.arg(old_key)
  AND NOT EXISTS (
    SELECT 1 FROM sys_translation occupied
    WHERE occupied.entity_key = sqlc.arg(new_key) AND occupied.lang = sys_translation.lang
  )
RETURNING sys_translation.entity_key;

-- name: GetPresetSourceRow :one
-- The row a preset save is about to rewrite: its current code and source text,
-- addressed by owning table and id. A save made in a target language keeps this
-- text in the column and files what the operator typed in sys_translation, and
-- a code edit moves the entity's records from the code above.
SELECT code, source, remark FROM v_preset_source_row
WHERE namespace = sqlc.arg(namespace) AND id = sqlc.arg(id);

-- name: CreateTranslation :exec
-- Administrator-created translation; a conflict refreshes the value so the
-- create form is safe to retry.
INSERT INTO sys_translation (entity_key, lang, value)
VALUES ($1, $2, $3)
ON CONFLICT (entity_key, lang) DO UPDATE
SET value = EXCLUDED.value, updated_at = NOW();

-- name: UpdateTranslation :execrows
-- Edits write straight through to sys_translation: every reader (sidebar,
-- role grants, menu management, ...) picks the new value up immediately. The
-- language can change too, so a record filed under the wrong language is a
-- simple edit; the caller checks (entity_key, lang) is still free first.
UPDATE sys_translation
SET lang = $2, value = $3, updated_at = NOW()
WHERE id = $1;

-- name: GetTranslationByID :one
SELECT id, entity_key, lang, value FROM sys_translation WHERE id = $1;

-- name: TranslationExists :one
-- Reports the id already occupying (entity_key, lang), so an edit can explain
-- the collision instead of hitting the unique constraint.
SELECT id FROM sys_translation WHERE entity_key = $1 AND lang = $2;

-- name: DeleteTranslationByID :execrows
-- Clearing a text deletes its record, so the row count is what tells an edit
-- apart from a no-op: zero means the record was already gone.
DELETE FROM sys_translation WHERE id = $1;

-- name: DeleteTranslation :execrows
-- Drops one entity's translation in one language. The count is returned so a
-- delete addressed by entity_key can answer 404 instead of "ok" for a row that
-- does not exist; the save path ignores it, because clearing an already-empty
-- language is a normal save, not an error.
DELETE FROM sys_translation WHERE entity_key = $1 AND lang = $2;
