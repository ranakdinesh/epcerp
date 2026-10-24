CREATE TABLE work_calendar_holidays(id uuid PRIMARY KEY,name text NOT NULL,holiday_date date NOT NULL UNIQUE,is_active boolean NOT NULL DEFAULT true,version integer NOT NULL DEFAULT 1 CHECK(version>0),created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id));
ALTER TABLE projects ADD COLUMN actual_end_date date, ADD COLUMN schedule_calculation jsonb;
CREATE TABLE work_calendar_changes(id uuid PRIMARY KEY,holiday_id uuid NOT NULL REFERENCES work_calendar_holidays(id),version integer NOT NULL,reason text NOT NULL,snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),UNIQUE(holiday_id,version));
CREATE FUNCTION reject_calendar_history_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Calendar history is immutable'; END $$;
CREATE TRIGGER calendar_history_immutable BEFORE UPDATE OR DELETE ON work_calendar_changes FOR EACH ROW EXECUTE FUNCTION reject_calendar_history_change();
CREATE TRIGGER calendar_history_no_truncate BEFORE TRUNCATE ON work_calendar_changes FOR EACH STATEMENT EXECUTE FUNCTION reject_calendar_history_change();
ALTER TABLE projects DROP CONSTRAINT project_capture_bounds;
ALTER TABLE projects ADD CONSTRAINT project_capture_bounds CHECK ((contract_value IS NULL OR contract_value>=0) AND (planned_duration_days IS NULL OR planned_duration_days BETWEEN 0 AND 36500) AND (safety_buffer_percent IS NULL OR safety_buffer_percent BETWEEN 0 AND 100) AND (safety_buffer_days IS NULL OR safety_buffer_days BETWEEN 0 AND 36500));

CREATE TABLE project_schedule_changes(id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),project_version integer NOT NULL,before_calculation jsonb,after_calculation jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),UNIQUE(project_id,project_version));
CREATE TRIGGER project_schedule_history_immutable BEFORE UPDATE OR DELETE ON project_schedule_changes FOR EACH ROW EXECUTE FUNCTION reject_calendar_history_change();
CREATE TRIGGER project_schedule_history_no_truncate BEFORE TRUNCATE ON project_schedule_changes FOR EACH STATEMENT EXECUTE FUNCTION reject_calendar_history_change();
