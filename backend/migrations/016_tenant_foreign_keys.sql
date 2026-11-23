-- Tenant-composite referential integrity.
--
-- Row-level security filters reads, but it says nothing about what a row may
-- point at. Before this migration a child row could reference a parent id that
-- belonged to another tenant: the insert passed the RLS WITH CHECK (the child's
-- own tenant_id was right) while the foreign key was satisfied by a parent in a
-- different tenant. The reference then either leaked a name through a join or
-- silently disappeared when the join was tenant-filtered.
--
-- The fix is a composite key. Each parent gets UNIQUE (tenant_id, id) and each
-- tenant-scoped child references (tenant_id, parent_id) instead of (parent_id),
-- which makes a cross-tenant reference impossible to store.
--
-- Idempotent: every step is guarded, so re-running is a no-op.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

-- Parent side: the pair (tenant_id, id) must be unique before a child can
-- reference it. Dropped first because Postgres has no ADD CONSTRAINT IF NOT
-- EXISTS.
ALTER TABLE sys_department DROP CONSTRAINT IF EXISTS uk_department_tenant_id;
ALTER TABLE sys_department ADD CONSTRAINT uk_department_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_position DROP CONSTRAINT IF EXISTS uk_position_tenant_id;
ALTER TABLE sys_position ADD CONSTRAINT uk_position_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_role DROP CONSTRAINT IF EXISTS uk_role_tenant_id;
ALTER TABLE sys_role ADD CONSTRAINT uk_role_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_dict_type DROP CONSTRAINT IF EXISTS uk_dict_type_tenant_id;
ALTER TABLE sys_dict_type ADD CONSTRAINT uk_dict_type_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_file DROP CONSTRAINT IF EXISTS uk_file_tenant_id;
ALTER TABLE sys_file ADD CONSTRAINT uk_file_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_notice DROP CONSTRAINT IF EXISTS uk_notice_tenant_id;
ALTER TABLE sys_notice ADD CONSTRAINT uk_notice_tenant_id UNIQUE (tenant_id, id);

ALTER TABLE sys_scheduled_job DROP CONSTRAINT IF EXISTS uk_scheduled_job_tenant_id;
ALTER TABLE sys_scheduled_job ADD CONSTRAINT uk_scheduled_job_tenant_id UNIQUE (tenant_id, id);

-- Child side: the reference now carries the tenant, so a parent in another
-- tenant cannot satisfy it.

-- A department's parent must live in the same tenant. parent_id 0 is the root
-- sentinel and has no row of its own, so a foreign key cannot express this;
-- a trigger enforces the rule for real parents and lets the sentinel through.
CREATE OR REPLACE FUNCTION app_check_department_parent_tenant()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.parent_id IS NULL OR NEW.parent_id = 0 THEN
    RETURN NEW;
  END IF;
  IF NOT EXISTS (
    SELECT 1 FROM sys_department parent
    WHERE parent.id = NEW.parent_id AND parent.tenant_id = NEW.tenant_id
  ) THEN
    RAISE EXCEPTION 'department % references parent % outside tenant %',
      NEW.id, NEW.parent_id, NEW.tenant_id
      USING ERRCODE = '23503';
  END IF;
  RETURN NEW;
END;
$$;

UPDATE sys_department child
SET parent_id = 0
WHERE parent_id <> 0
  AND NOT EXISTS (
    SELECT 1 FROM sys_department parent
    WHERE parent.id = child.parent_id AND parent.tenant_id = child.tenant_id
  );

DROP TRIGGER IF EXISTS trg_department_parent_tenant ON sys_department;
CREATE TRIGGER trg_department_parent_tenant
  BEFORE INSERT OR UPDATE OF parent_id, tenant_id ON sys_department
  FOR EACH ROW EXECUTE FUNCTION app_check_department_parent_tenant();

ALTER TABLE sys_user_role DROP CONSTRAINT IF EXISTS fk_user_role_role_tenant;
DELETE FROM sys_user_role ur
WHERE NOT EXISTS (
  SELECT 1 FROM sys_role r WHERE r.id = ur.role_id AND r.tenant_id = ur.tenant_id
);
ALTER TABLE sys_user_role
  ADD CONSTRAINT fk_user_role_role_tenant
  FOREIGN KEY (tenant_id, role_id) REFERENCES sys_role (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_user_position DROP CONSTRAINT IF EXISTS fk_user_position_position_tenant;
DELETE FROM sys_user_position up
WHERE NOT EXISTS (
  SELECT 1 FROM sys_position p WHERE p.id = up.position_id AND p.tenant_id = up.tenant_id
);
ALTER TABLE sys_user_position
  ADD CONSTRAINT fk_user_position_position_tenant
  FOREIGN KEY (tenant_id, position_id) REFERENCES sys_position (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_role_department DROP CONSTRAINT IF EXISTS fk_role_department_department_tenant;
DELETE FROM sys_role_department rd
WHERE NOT EXISTS (
  SELECT 1 FROM sys_department d WHERE d.id = rd.department_id AND d.tenant_id = rd.tenant_id
);
ALTER TABLE sys_role_department
  ADD CONSTRAINT fk_role_department_department_tenant
  FOREIGN KEY (tenant_id, department_id) REFERENCES sys_department (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_dict_item DROP CONSTRAINT IF EXISTS fk_dict_item_type_tenant;
DELETE FROM sys_dict_item di
WHERE NOT EXISTS (
  SELECT 1 FROM sys_dict_type dt WHERE dt.id = di.dict_type_id AND dt.tenant_id = di.tenant_id
);
ALTER TABLE sys_dict_item
  ADD CONSTRAINT fk_dict_item_type_tenant
  FOREIGN KEY (tenant_id, dict_type_id) REFERENCES sys_dict_type (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_file_reference DROP CONSTRAINT IF EXISTS fk_file_reference_file_tenant;
DELETE FROM sys_file_reference fr
WHERE NOT EXISTS (
  SELECT 1 FROM sys_file f WHERE f.id = fr.file_id AND f.tenant_id = fr.tenant_id
);
ALTER TABLE sys_file_reference
  ADD CONSTRAINT fk_file_reference_file_tenant
  FOREIGN KEY (tenant_id, file_id) REFERENCES sys_file (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_notice_read DROP CONSTRAINT IF EXISTS fk_notice_read_notice_tenant;
DELETE FROM sys_notice_read nr
WHERE NOT EXISTS (
  SELECT 1 FROM sys_notice n WHERE n.id = nr.notice_id AND n.tenant_id = nr.tenant_id
);
ALTER TABLE sys_notice_read
  ADD CONSTRAINT fk_notice_read_notice_tenant
  FOREIGN KEY (tenant_id, notice_id) REFERENCES sys_notice (tenant_id, id)
  ON DELETE CASCADE;

ALTER TABLE sys_job_log DROP CONSTRAINT IF EXISTS fk_job_log_job_tenant;
DELETE FROM sys_job_log jl
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job j WHERE j.id = jl.job_id AND j.tenant_id = jl.tenant_id
);
ALTER TABLE sys_job_log
  ADD CONSTRAINT fk_job_log_job_tenant
  FOREIGN KEY (tenant_id, job_id) REFERENCES sys_scheduled_job (tenant_id, id)
  ON DELETE CASCADE;

COMMIT;
