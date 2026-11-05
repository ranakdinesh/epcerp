BEGIN;
CREATE TABLE project_budgets (
 project_id uuid PRIMARY KEY REFERENCES projects(id),version integer NOT NULL CHECK(version>0),revision integer NOT NULL CHECK(revision>0),state varchar(20) NOT NULL CHECK(state='draft'),
 total numeric(16,2) NOT NULL CHECK(total>=0),currency char(3) NOT NULL,owner_id uuid NOT NULL REFERENCES users(id),notes varchar(2000) NOT NULL,pricing_mode varchar(20) NOT NULL CHECK(pricing_mode IN('','unit_rates')),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id)
);
CREATE TABLE project_budget_heads (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES project_budgets(project_id),sort_order integer NOT NULL CHECK(sort_order>=0),parent_id uuid,name varchar(160) NOT NULL,kind varchar(30) NOT NULL,amount numeric(16,2) NOT NULL CHECK(amount>=0),owner_id uuid NOT NULL REFERENCES users(id),is_active boolean NOT NULL,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),UNIQUE(id,project_id),
 FOREIGN KEY(parent_id,project_id) REFERENCES project_budget_heads(id,project_id) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX project_budget_head_project ON project_budget_heads(project_id);
CREATE TABLE project_budget_allocations (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES project_budgets(project_id),sort_order integer NOT NULL CHECK(sort_order>=0),bo_id uuid NOT NULL REFERENCES purchase_bo_lists(id),line_id uuid NOT NULL REFERENCES purchase_bo_lines(id),head_id uuid NOT NULL,
 amount numeric(16,2) NOT NULL CHECK(amount>=0),owner_id uuid NOT NULL REFERENCES users(id),is_active boolean NOT NULL,unit_price numeric(16,2) CHECK(unit_price>=0),pricing_quantity numeric(18,4) CHECK(pricing_quantity>0),source_snapshot jsonb NOT NULL,
 created_at timestamptz NOT NULL,updated_at timestamptz NOT NULL,created_by_id uuid REFERENCES users(id),updated_by_id uuid REFERENCES users(id),
 FOREIGN KEY(head_id,project_id) REFERENCES project_budget_heads(id,project_id),CHECK((unit_price IS NULL)=(pricing_quantity IS NULL))
);
CREATE UNIQUE INDEX project_budget_active_item ON project_budget_allocations(project_id,bo_id,line_id) WHERE is_active;
CREATE TABLE project_budget_events (
 id uuid PRIMARY KEY,project_id uuid NOT NULL REFERENCES project_budgets(project_id),action varchar(30) NOT NULL CHECK(action IN('save','price_items')),reason varchar(2000) NOT NULL,
 before_snapshot jsonb NOT NULL,after_snapshot jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),created_by_id uuid REFERENCES users(id)
);
CREATE INDEX project_budget_history ON project_budget_events(project_id,created_at,id);
CREATE FUNCTION protect_project_budget_evidence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Budget evidence is immutable; deactivate records instead';END $$;
CREATE TRIGGER project_budgets_no_delete BEFORE DELETE ON project_budgets FOR EACH ROW EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budgets_no_truncate BEFORE TRUNCATE ON project_budgets FOR EACH STATEMENT EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_heads_no_delete BEFORE DELETE ON project_budget_heads FOR EACH ROW EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_heads_no_truncate BEFORE TRUNCATE ON project_budget_heads FOR EACH STATEMENT EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_allocations_no_delete BEFORE DELETE ON project_budget_allocations FOR EACH ROW EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_allocations_no_truncate BEFORE TRUNCATE ON project_budget_allocations FOR EACH STATEMENT EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_events_immutable BEFORE UPDATE OR DELETE ON project_budget_events FOR EACH ROW EXECUTE FUNCTION protect_project_budget_evidence();
CREATE TRIGGER project_budget_events_no_truncate BEFORE TRUNCATE ON project_budget_events FOR EACH STATEMENT EXECUTE FUNCTION protect_project_budget_evidence();
CREATE FUNCTION protect_project_budget_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Budget identity is immutable';END IF;
 IF NEW.version<>OLD.version+1 THEN RAISE EXCEPTION 'Budget requires the next version';END IF;RETURN NEW;END $$;
CREATE TRIGGER project_budget_identity BEFORE UPDATE ON project_budgets FOR EACH ROW EXECUTE FUNCTION protect_project_budget_identity();
CREATE FUNCTION protect_project_budget_child_identity() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF ROW(NEW.id,NEW.project_id,NEW.created_at,NEW.created_by_id) IS DISTINCT FROM ROW(OLD.id,OLD.project_id,OLD.created_at,OLD.created_by_id) THEN RAISE EXCEPTION 'Budget child identity is immutable';END IF;RETURN NEW;END $$;
CREATE TRIGGER project_budget_head_identity BEFORE UPDATE ON project_budget_heads FOR EACH ROW EXECUTE FUNCTION protect_project_budget_child_identity();
CREATE TRIGGER project_budget_allocation_identity BEFORE UPDATE ON project_budget_allocations FOR EACH ROW EXECUTE FUNCTION protect_project_budget_child_identity();
CREATE FUNCTION protect_project_budget_item_scope() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 IF TG_OP='UPDATE' AND ROW(NEW.bo_id,NEW.line_id) IS DISTINCT FROM ROW(OLD.bo_id,OLD.line_id) THEN RAISE EXCEPTION 'Allocation source identity is immutable';END IF;
 IF NOT EXISTS(SELECT 1 FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id WHERE b.id=NEW.bo_id AND b.project_id=NEW.project_id AND l.id=NEW.line_id) THEN RAISE EXCEPTION 'Budget allocation belongs to a different project or BO';END IF;RETURN NEW;END $$;
CREATE TRIGGER project_budget_allocation_scope BEFORE INSERT OR UPDATE ON project_budget_allocations FOR EACH ROW EXECUTE FUNCTION protect_project_budget_item_scope();
COMMIT;
