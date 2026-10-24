CREATE TABLE purchase_work_packages(
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES projects(id),indent_id uuid NOT NULL REFERENCES purchase_indents(id),demand_id uuid NOT NULL REFERENCES project_material_requirements(id),code text NOT NULL UNIQUE,description text NOT NULL,source_snapshot jsonb NOT NULL,steps jsonb NOT NULL CHECK(jsonb_typeof(steps)='array' AND jsonb_array_length(steps)=20),version integer NOT NULL CHECK(version>0),is_active boolean NOT NULL DEFAULT true,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(project_id,indent_id,demand_id)
);
CREATE INDEX purchase_work_packages_project ON purchase_work_packages(project_id,created_at,id);
CREATE TABLE purchase_work_package_events(id uuid PRIMARY KEY,subject_id uuid NOT NULL REFERENCES purchase_work_packages(id),action text NOT NULL CHECK(action IN('create','assign','save','start','block','resolve')),reason text NOT NULL CHECK(length(btrim(reason))>0),before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id));
CREATE FUNCTION protect_purchase_work_package()RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE step jsonb;prior jsonb;i integer:=0;
BEGIN
 IF TG_OP='UPDATE' AND(NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.indent_id<>OLD.indent_id OR NEW.demand_id<>OLD.demand_id OR NEW.code<>OLD.code OR NEW.description<>OLD.description OR NEW.source_snapshot<>OLD.source_snapshot OR NEW.created_at<>OLD.created_at OR NEW.created_by_id IS DISTINCT FROM OLD.created_by_id OR NEW.version<>OLD.version+1)THEN RAISE EXCEPTION 'Package identity/source/version is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_indents WHERE id=NEW.indent_id AND project_id=NEW.project_id) OR NOT EXISTS(SELECT 1 FROM project_material_requirements WHERE id=NEW.demand_id AND project_id=NEW.project_id) OR NOT EXISTS(SELECT 1 FROM purchase_indent_releases WHERE indent_id=NEW.indent_id AND demand_id=NEW.demand_id) THEN RAISE EXCEPTION 'Package must reference its released project indent line';END IF;
 FOR step IN SELECT value FROM jsonb_array_elements(NEW.steps)LOOP
 i:=i+1;
 IF (step->>'number')::integer IS DISTINCT FROM i OR step->>'state' IS NULL OR(i<=2 AND step->>'state'<>'source_linked')OR(i>2 AND step->>'state' NOT IN('not_started','preparing','blocked'))THEN RAISE EXCEPTION 'Invalid purchase preparation catalogue/state';END IF;
 IF TG_OP='UPDATE'THEN prior:=OLD.steps->(i-1);IF step->'number' IS DISTINCT FROM prior->'number' OR step->'title' IS DISTINCT FROM prior->'title' OR step->'phase' IS DISTINCT FROM prior->'phase' OR step->'evidence_required' IS DISTINCT FROM prior->'evidence_required'THEN RAISE EXCEPTION 'Frozen step definition is immutable';END IF;END IF;
 END LOOP;RETURN NEW;
END $$;
CREATE TRIGGER purchase_work_package_guard BEFORE INSERT OR UPDATE ON purchase_work_packages FOR EACH ROW EXECUTE FUNCTION protect_purchase_work_package();
CREATE FUNCTION refuse_purchase_work_evidence_mutation()RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'Purchase package evidence cannot be removed or rewritten';END $$;
CREATE TRIGGER purchase_work_packages_no_delete BEFORE DELETE ON purchase_work_packages FOR EACH ROW EXECUTE FUNCTION refuse_purchase_work_evidence_mutation();
CREATE TRIGGER purchase_work_packages_no_truncate BEFORE TRUNCATE ON purchase_work_packages FOR EACH STATEMENT EXECUTE FUNCTION refuse_purchase_work_evidence_mutation();
CREATE TRIGGER purchase_work_events_immutable BEFORE UPDATE OR DELETE ON purchase_work_package_events FOR EACH ROW EXECUTE FUNCTION refuse_purchase_work_evidence_mutation();
CREATE TRIGGER purchase_work_events_no_truncate BEFORE TRUNCATE ON purchase_work_package_events FOR EACH STATEMENT EXECUTE FUNCTION refuse_purchase_work_evidence_mutation();
