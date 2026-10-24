-- Go-owned client_references schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE client_types (
	id UUID NOT NULL, 
	name VARCHAR(128) NOT NULL, 
	normalized_name VARCHAR(128) NOT NULL, 
	code VARCHAR(32) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	CONSTRAINT uq_client_types_normalized_name UNIQUE (normalized_name), 
	CONSTRAINT uq_client_types_code UNIQUE (code), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE industries (
	id UUID NOT NULL, 
	name VARCHAR(128) NOT NULL, 
	normalized_name VARCHAR(128) NOT NULL, 
	code VARCHAR(32) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	CONSTRAINT uq_industries_normalized_name UNIQUE (normalized_name), 
	CONSTRAINT uq_industries_code UNIQUE (code), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE indian_states (
	id UUID NOT NULL, 
	name VARCHAR(128) NOT NULL, 
	normalized_name VARCHAR(128) NOT NULL, 
	code VARCHAR(32) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	CONSTRAINT uq_indian_states_normalized_name UNIQUE (normalized_name), 
	CONSTRAINT uq_indian_states_code UNIQUE (code), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE indian_cities (
	state_id UUID NOT NULL, 
	id UUID NOT NULL, 
	name VARCHAR(128) NOT NULL, 
	normalized_name VARCHAR(128) NOT NULL, 
	code VARCHAR(32) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	CONSTRAINT uq_indian_cities_state_normalized_name UNIQUE (state_id, normalized_name), 
	FOREIGN KEY(state_id) REFERENCES indian_states (id) ON DELETE RESTRICT, 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;
COMMIT;
