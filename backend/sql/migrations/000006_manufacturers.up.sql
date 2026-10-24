-- Go-owned manufacturers schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE manufacturers (
	id UUID NOT NULL, 
	code VARCHAR(32) NOT NULL, 
	name VARCHAR(255) NOT NULL, 
	data JSONB NOT NULL, 
	is_active BOOLEAN DEFAULT 'true' NOT NULL, 
	version INTEGER DEFAULT '1' NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	UNIQUE (code), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE manufacturers_events (
	id UUID NOT NULL, 
	subject_id UUID NOT NULL, 
	action VARCHAR(40) NOT NULL, 
	reason VARCHAR(2000) NOT NULL, 
	"before" JSONB NOT NULL, 
	"after" JSONB NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	PRIMARY KEY (id), 
	FOREIGN KEY(subject_id) REFERENCES manufacturers (id), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;
ALTER TABLE manufacturers ADD CONSTRAINT manufacturers_version_positive CHECK(version > 0);
CREATE INDEX manufacturers_events_subject_id_idx ON manufacturers_events(subject_id, created_at);
COMMIT;
