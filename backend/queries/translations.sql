-- name: ListTranslationsByKey :many
-- A tenant row overrides the platform default (tenant_id 0) for the same
-- (entity_key, lang). DISTINCT ON keeps the highest tenant_id, which is the
-- caller's own row whenever one exists.
-- The scope predicate repeats what the RLS policy enforces. It is deliberate:
-- a connection made as a SUPERUSER or BYPASSRLS role silently disables the
-- policy, and then "highest tenant_id" would hand one tenant another's label.
SELECT DISTINCT ON (entity_key) entity_key, value
FROM sys_translation
WHERE lang = $1 AND entity_key = ANY($2::text[])
  AND tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
ORDER BY entity_key, tenant_id DESC;

-- name: UpsertTranslationIfAbsent :exec
-- Seed-time insert only: an administrator-edited translation (or one written
-- by a previous run) must never be clobbered by the compiled-in default.
-- Bootstrap runs without a session tenant, so the platform row (0) is written.
INSERT INTO sys_translation (tenant_id, entity_key, lang, value)
VALUES (COALESCE(app_current_tenant_id(), 0), $1, $2, $3)
ON CONFLICT (tenant_id, entity_key, lang) DO NOTHING;

-- name: ListTranslationRows :many
-- Paged translation records with their Chinese source text, for the
-- translation management page. Only rows that exist are listed; entities
-- without a translation show up in ListUntranslatedEntities instead.
-- Every row is validated against v_preset_entity, the single catalog of
-- translatable preset entities (see schema/tables.sql).
-- visible collapses the two scopes: the caller's row wins over the platform
-- default for the same (entity_key, lang). tenant_id is returned so the page
-- can mark inherited rows and refuse to delete a platform default from a
-- tenant session.
WITH visible AS (
  SELECT DISTINCT ON (entity_key, lang)
         id, tenant_id, entity_key, lang, value, updated_at
  FROM sys_translation
  WHERE tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
  ORDER BY entity_key, lang, tenant_id DESC
)
SELECT v.id, v.tenant_id, v.entity_key::text AS entity_key,
       e.source::text AS source, v.value, v.lang, v.updated_at
FROM visible v
JOIN v_preset_entity e ON e.entity_key = v.entity_key
WHERE (sqlc.arg(lang)::text = '' OR v.lang = sqlc.arg(lang))
  AND (
    sqlc.arg(keyword)::text = ''
    OR v.entity_key ILIKE sqlc.arg(keyword)::text
    OR e.source ILIKE sqlc.arg(keyword)::text
    OR v.value ILIKE sqlc.arg(keyword)::text
  )
ORDER BY v.entity_key, v.lang
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: CountTranslationRows :one
WITH visible AS (
  SELECT DISTINCT ON (entity_key, lang) entity_key, lang, value
  FROM sys_translation
  WHERE tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
  ORDER BY entity_key, lang, tenant_id DESC
)
SELECT count(*)
FROM visible v
JOIN v_preset_entity e ON e.entity_key = v.entity_key
WHERE (sqlc.arg(lang)::text = '' OR v.lang = sqlc.arg(lang))
  AND (
    sqlc.arg(keyword)::text = ''
    OR v.entity_key ILIKE sqlc.arg(keyword)::text
    OR e.source ILIKE sqlc.arg(keyword)::text
    OR v.value ILIKE sqlc.arg(keyword)::text
  );

-- name: ListUntranslatedEntities :many
-- Preset entities that have no translation in the given language; these are
-- the choices offered by the "new translation" form. A platform default counts
-- as translated — RLS has already narrowed the scan to the visible scopes.
SELECT e.entity_key::text AS entity_key, e.source::text AS source
FROM v_preset_entity e
WHERE NOT EXISTS (
    SELECT 1 FROM sys_translation t
    WHERE t.entity_key = e.entity_key AND t.lang = $1
      AND t.tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
  )
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
-- namespace filters by entity_key prefix (menu/perm/role/...); empty means all.
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
  AND (sqlc.arg(namespace)::text = ''
       OR e.entity_key LIKE sqlc.arg(namespace)::text || '.%')
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

-- name: SetDictTypeSourceText :execrows
-- The dicttype.<code> key addresses the dictionary type of the caller's
-- tenant (row-level security scopes the read); renaming writes the label back
-- into the owning column so Chinese stays the source of truth.
UPDATE sys_dict_type SET name = sqlc.arg(value), updated_at = NOW()
WHERE code = sqlc.arg(code) AND deleted_at IS NULL;

-- name: SetDictItemSourceText :execrows
-- The dict.<type code>.<value> key addresses one item of the named type.
-- The value half of the key is the item's business value, not a code, so the
-- lookup goes through the owning type instead of a rekey.
UPDATE sys_dict_item SET label = sqlc.arg(value), updated_at = NOW()
WHERE dict_type_id = (
    SELECT id FROM sys_dict_type
    WHERE code = sqlc.arg(type_code) AND deleted_at IS NULL
  )
  AND value = sqlc.arg(item_value) AND deleted_at IS NULL;

-- name: RekeyDictTranslationsByPrefix :execrows
-- A dictionary type's code is the prefix of every item key
-- (dict.<code>.<value>), so renaming a type moves all of them at once. Keys
-- whose destination is already taken are left behind, matching
-- RekeyTranslations. The prefix compare is left() rather than LIKE so a code
-- containing _ or % (legal since codes are snake_case) cannot over-match.
UPDATE sys_translation AS tr SET entity_key = sqlc.arg(new_prefix) || substring(tr.entity_key from length(sqlc.arg(old_prefix))::int + 1), updated_at = NOW()
WHERE left(tr.entity_key, length(sqlc.arg(old_prefix))::int) = sqlc.arg(old_prefix)
  AND tr.tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
  AND NOT EXISTS (
    SELECT 1 FROM sys_translation occupied
    WHERE occupied.tenant_id = tr.tenant_id
      AND occupied.entity_key = sqlc.arg(new_prefix) || substring(tr.entity_key from length(sqlc.arg(old_prefix))::int + 1)
      AND occupied.lang = tr.lang
  );

-- name: RekeyTranslations :many
-- A translation record belongs to the entity, not to its code, so editing a
-- code carries the records over instead of orphaning them (an orphaned key is
-- invisible: every list joins v_preset_entity). Rows whose destination is
-- already taken are left alone rather than violating the tenant-scoped unique
-- key. The scope is part of the conflict test: the platform default and a
-- tenant override are separate rows that happen to share a key.
UPDATE sys_translation AS tr SET entity_key = sqlc.arg(new_key), updated_at = NOW()
WHERE tr.entity_key = sqlc.arg(old_key)
  AND tr.tenant_id IN (0, COALESCE(app_current_tenant_id(), 0))
  AND NOT EXISTS (
    SELECT 1 FROM sys_translation occupied
    WHERE occupied.tenant_id = tr.tenant_id
      AND occupied.entity_key = sqlc.arg(new_key)
      AND occupied.lang = tr.lang
  )
RETURNING tr.entity_key;

-- name: GetPresetSourceRow :one
-- The row a preset save is about to rewrite: its current code and source text,
-- addressed by owning table and id. A save made in a target language keeps this
-- text in the column and files what the operator typed in sys_translation, and
-- a code edit moves the entity's records from the code above.
SELECT code, source, remark FROM v_preset_source_row
WHERE namespace = sqlc.arg(namespace) AND id = sqlc.arg(id);

-- name: CreateTranslation :exec
-- Administrator-created translation; a conflict refreshes the value so the
-- create form is safe to retry. The row belongs to the caller's tenant
-- (platform 0 when no tenant session is set, e.g. during bootstrap).
INSERT INTO sys_translation (tenant_id, entity_key, lang, value)
VALUES (COALESCE(app_current_tenant_id(), 0), $1, $2, $3)
ON CONFLICT (tenant_id, entity_key, lang) DO UPDATE
SET value = EXCLUDED.value, updated_at = NOW();

-- name: UpdateTranslation :execrows
-- Edits write straight through to sys_translation: every reader (sidebar,
-- role grants, menu management, ...) picks the new value up immediately. The
-- language can change too, so a record filed under the wrong language is a
-- simple edit; the caller checks (entity_key, lang) is still free first.
-- A platform default (tenant 0) is never rewritten from a tenant session: the
-- store copies it into a tenant override instead, and this predicate is the
-- second line of defence.
UPDATE sys_translation
SET lang = $2, value = $3, updated_at = NOW()
WHERE id = $1
  AND tenant_id = COALESCE(app_current_tenant_id(), 0);

-- name: GetTranslationByID :one
SELECT id, tenant_id, entity_key, lang, value FROM sys_translation WHERE id = $1;

-- name: TranslationExists :one
-- Reports the id already occupying (entity_key, lang) in the caller's scope,
-- so an edit can explain the collision instead of hitting the unique
-- constraint.
SELECT id FROM sys_translation
WHERE entity_key = $1 AND lang = $2
  AND tenant_id = COALESCE(app_current_tenant_id(), 0);

-- name: PlatformTranslationExists :one
-- Used after a delete matched no row: it tells "already gone" apart from
-- "inherited platform default", which the caller may not remove. The read
-- policy makes tenant 0 rows visible everywhere, so this answers for a tenant
-- session too.
SELECT id FROM sys_translation
WHERE entity_key = $1 AND lang = $2 AND tenant_id = 0;

-- name: DeleteTranslationByID :execrows
-- Clearing a text deletes its record, so the row count is what tells an edit
-- apart from a no-op: zero means the record was already gone — or that it is a
-- platform default the caller may not remove.
DELETE FROM sys_translation
WHERE id = $1
  AND tenant_id = COALESCE(app_current_tenant_id(), 0);

-- name: DeleteTranslation :execrows
-- Drops one entity's translation in one language. The count is returned so a
-- delete addressed by entity_key can answer 404 instead of "ok" for a row that
-- does not exist; the save path ignores it, because clearing an already-empty
-- language is a normal save, not an error. Scoped to the caller's own rows, so
-- a tenant cannot erase the platform default it inherits.
DELETE FROM sys_translation
WHERE entity_key = $1 AND lang = $2
  AND tenant_id = COALESCE(app_current_tenant_id(), 0);
