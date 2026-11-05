BEGIN;
CREATE TABLE purchase_technical_cases(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),package_id uuid NOT NULL UNIQUE REFERENCES purchase_work_packages(id),source_case_id uuid NOT NULL REFERENCES purchase_sourcing_cases(id),
 version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),state varchar(30) NOT NULL CHECK(state IN('draft','submitted','under_review','approved','approved_with_comments','revision_required')),
 source_snapshot jsonb NOT NULL CHECK(jsonb_typeof(source_snapshot)='object'),data jsonb NOT NULL CHECK(jsonb_typeof(data)='object'),is_active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(id,project_id)
);
CREATE TABLE purchase_technical_events(
 id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_technical_cases(id),action varchar(40) NOT NULL,reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),
 before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id)
);
CREATE INDEX purchase_technical_history ON purchase_technical_events(subject_id,created_at DESC);
CREATE TABLE purchase_technical_files(
 id uuid PRIMARY KEY,case_id uuid NOT NULL REFERENCES purchase_technical_cases(id),purpose varchar(30) NOT NULL CHECK(purpose IN('document','transmittal','client_approval','conditions')),
 name varchar(255) NOT NULL,media_type varchar(40) NOT NULL CHECK(media_type IN('application/pdf','image/png','image/jpeg')),byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 10485760),sha256 char(64) NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),ciphertext bytea NOT NULL CHECK(octet_length(ciphertext)>byte_size),
 is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(id,case_id)
);
CREATE TABLE purchase_technical_file_events(
 id uuid PRIMARY KEY,case_id uuid NOT NULL REFERENCES purchase_technical_cases(id),file_id uuid NOT NULL,action varchar(20) NOT NULL CHECK(action IN('uploaded','downloaded')),reason text NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 2000),created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),FOREIGN KEY(file_id,case_id) REFERENCES purchase_technical_files(id,case_id)
);
CREATE FUNCTION protect_purchase_technical() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_sourcing_cases s WHERE s.id=NEW.source_case_id AND s.project_id=NEW.project_id AND s.package_id=NEW.package_id AND s.state='released' AND s.is_active AND NEW.source_snapshot->>'case_id'=s.id::text AND NEW.source_snapshot->>'version'=s.version::text AND NEW.source_snapshot->>'loi_reference'=s.data->>'loi_reference') THEN RAISE EXCEPTION 'Technical kit requires its exact released project LOI';END IF;
 IF NEW.data->>'id' IS DISTINCT FROM NEW.id::text OR NEW.data->>'project_id' IS DISTINCT FROM NEW.project_id::text OR NEW.data->>'package_id' IS DISTINCT FROM NEW.package_id::text OR NEW.data->>'version' IS DISTINCT FROM NEW.version::text OR NEW.data->>'revision' IS DISTINCT FROM NEW.revision::text OR NEW.data->>'state' IS DISTINCT FROM NEW.state OR NEW.data->>'is_active' IS DISTINCT FROM NEW.is_active::text OR NEW.data->'source_snapshot' IS DISTINCT FROM NEW.source_snapshot THEN RAISE EXCEPTION 'Technical state projection mismatch';END IF;
 IF jsonb_typeof(NEW.data->'documents') IS DISTINCT FROM 'array' OR jsonb_array_length(NEW.data->'documents')<>6 THEN RAISE EXCEPTION 'Technical kit requires its six owned checklist records';END IF;
 IF TG_OP='INSERT' AND (NEW.version<>1 OR NEW.revision<>1 OR NEW.state<>'draft') THEN RAISE EXCEPTION 'Technical kits begin in pending draft';END IF;
 IF TG_OP='UPDATE' AND (NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.package_id<>OLD.package_id OR NEW.source_case_id<>OLD.source_case_id OR NEW.source_snapshot IS DISTINCT FROM OLD.source_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1 OR NEW.revision<OLD.revision OR NEW.revision>OLD.revision+1) THEN RAISE EXCEPTION 'Technical ownership, revision and audit history must be retained';END IF;
 RETURN NEW;END $$;
CREATE TRIGGER purchase_technical_guard BEFORE INSERT OR UPDATE ON purchase_technical_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_technical();
CREATE FUNCTION protect_purchase_technical_history() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Technical evidence and history are immutable';END $$;
CREATE TRIGGER purchase_technical_no_delete BEFORE DELETE ON purchase_technical_cases FOR EACH ROW EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_no_truncate BEFORE TRUNCATE ON purchase_technical_cases FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_event_immutable BEFORE UPDATE OR DELETE ON purchase_technical_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_event_no_truncate BEFORE TRUNCATE ON purchase_technical_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_file_immutable BEFORE UPDATE OR DELETE ON purchase_technical_files FOR EACH ROW EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_file_no_truncate BEFORE TRUNCATE ON purchase_technical_files FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_file_event_immutable BEFORE UPDATE OR DELETE ON purchase_technical_file_events FOR EACH ROW EXECUTE FUNCTION protect_purchase_technical_history();
CREATE TRIGGER purchase_technical_file_event_no_truncate BEFORE TRUNCATE ON purchase_technical_file_events FOR EACH STATEMENT EXECUTE FUNCTION protect_purchase_technical_history();
CREATE FUNCTION check_purchase_technical_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_events WHERE subject_id=NEW.id AND after_snapshot=NEW.data AND created_by_id IS NOT DISTINCT FROM NEW.updated_by_id AND created_at=NEW.updated_at AND (TG_OP='INSERT' OR before_snapshot=OLD.data)) THEN RAISE EXCEPTION 'Technical write requires exact immutable audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_technical_write_integrity AFTER INSERT OR UPDATE ON purchase_technical_cases DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_technical_write();
CREATE FUNCTION check_purchase_technical_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_technical_cases WHERE id=NEW.subject_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_technical' AND entity_type='technical_case' AND entity_id=NEW.subject_id AND entity_version=(NEW.after_snapshot->>'version')::integer AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Technical audit requires immutable central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_technical_event_integrity AFTER INSERT ON purchase_technical_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_technical_event();
CREATE FUNCTION check_purchase_technical_file() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM purchase_technical_file_events WHERE file_id=NEW.id AND case_id=NEW.case_id AND action='uploaded' AND created_at=NEW.created_at AND created_by_id IS NOT DISTINCT FROM NEW.created_by_id) THEN RAISE EXCEPTION 'Technical upload requires immutable file audit';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_technical_file_integrity AFTER INSERT ON purchase_technical_files DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_technical_file();
CREATE FUNCTION check_purchase_technical_file_event() RETURNS trigger LANGUAGE plpgsql AS $$ DECLARE p uuid;BEGIN
 SELECT project_id INTO p FROM purchase_technical_cases WHERE id=NEW.case_id;
 IF NOT EXISTS(SELECT 1 FROM business_events WHERE id=NEW.id AND source_module='purchase_technical' AND entity_type='technical_file_event' AND entity_id=NEW.id AND entity_version=1 AND action=NEW.action AND actor_id=NEW.created_by_id AND actor_kind='user' AND scope_kind='project' AND scope_id=p AND visibility='restricted') THEN RAISE EXCEPTION 'Technical file audit requires central project event';END IF;
 RETURN NEW;END $$;
CREATE CONSTRAINT TRIGGER purchase_technical_file_event_integrity AFTER INSERT ON purchase_technical_file_events DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_purchase_technical_file_event();
COMMIT;
