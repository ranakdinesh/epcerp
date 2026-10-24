-- Go-owned clients schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE clients (
	id UUID NOT NULL, 
	client_code VARCHAR(20) NOT NULL, 
	legal_name VARCHAR(255) NOT NULL, 
	normalized_legal_name VARCHAR(255) NOT NULL, 
	display_name VARCHAR(255) NOT NULL, 
	parent_organization VARCHAR(255) NOT NULL, 
	client_type VARCHAR(64) NOT NULL, 
	industry VARCHAR(128) NOT NULL, 
	gstin VARCHAR(15) NOT NULL, 
	pan VARCHAR(10) NOT NULL, 
	cin VARCHAR(21) NOT NULL, 
	website VARCHAR(255) NOT NULL, 
	address_line_1 VARCHAR(255) NOT NULL, 
	address_line_2 VARCHAR(255) NOT NULL, 
	city VARCHAR(128) NOT NULL, 
	state VARCHAR(128) NOT NULL, 
	postal_code VARCHAR(24) NOT NULL, 
	country VARCHAR(128) NOT NULL, 
	legacy_source VARCHAR(128) NOT NULL, 
	legacy_source_id VARCHAR(64) NOT NULL, 
	legacy_source_row INTEGER, 
	legacy_created_at TIMESTAMP WITH TIME ZONE, 
	legacy_updated_at TIMESTAMP WITH TIME ZONE, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE client_contacts (
	id UUID NOT NULL, 
	client_id UUID NOT NULL, 
	full_name VARCHAR(128) NOT NULL, 
	designation VARCHAR(128) NOT NULL, 
	department VARCHAR(128) NOT NULL, 
	email VARCHAR(255) NOT NULL, 
	phone VARCHAR(32) NOT NULL, 
	is_primary BOOLEAN NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	FOREIGN KEY(client_id) REFERENCES clients (id) ON DELETE CASCADE, 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;
CREATE UNIQUE INDEX clients_client_code_key ON clients(client_code);
CREATE UNIQUE INDEX clients_normalized_legal_name_key ON clients(normalized_legal_name);
CREATE INDEX client_contacts_client_id_idx ON client_contacts(client_id);
COMMIT;
