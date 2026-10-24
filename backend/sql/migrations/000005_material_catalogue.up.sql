-- Go-owned material_catalogue schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE material_catalogue (
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
ALTER TABLE material_catalogue ADD CONSTRAINT material_catalogue_version_positive CHECK(version > 0);
COMMIT;
