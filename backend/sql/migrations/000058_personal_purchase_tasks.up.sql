BEGIN;
CREATE INDEX purchase_work_packages_assignment_lookup ON purchase_work_packages USING gin(steps jsonb_path_ops) WHERE is_active;
COMMIT;
