-- Go-owned storage schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE stored_documents (
	id UUID NOT NULL, 
	entity_type VARCHAR(60) NOT NULL, 
	entity_id UUID NOT NULL, 
	doc_type VARCHAR(60) NOT NULL, 
	title VARCHAR(255) NOT NULL, 
	original_filename VARCHAR(255) NOT NULL, 
	content_type VARCHAR(120) NOT NULL, 
	byte_size INTEGER NOT NULL, 
	storage_bucket VARCHAR(100) NOT NULL, 
	storage_key VARCHAR(500) NOT NULL, 
	sha256_hash VARCHAR(64) NOT NULL, 
	is_encrypted BOOLEAN DEFAULT 'true' NOT NULL, 
	encryption_algorithm VARCHAR(50) DEFAULT 'AES-256-GCM' NOT NULL, 
	is_active BOOLEAN DEFAULT 'true' NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE stored_document_events (
	id UUID NOT NULL, 
	document_id UUID NOT NULL, 
	action VARCHAR(50) NOT NULL, 
	actor_id UUID, 
	metadata_snapshot JSONB DEFAULT '{}'::jsonb NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	PRIMARY KEY (id), 
	FOREIGN KEY(document_id) REFERENCES stored_documents (id) ON DELETE CASCADE, 
	FOREIGN KEY(actor_id) REFERENCES users (id) ON DELETE SET NULL
)

;
CREATE UNIQUE INDEX stored_documents_object_key ON stored_documents(storage_bucket, storage_key);
CREATE INDEX stored_documents_entity_idx ON stored_documents(entity_type, entity_id);
CREATE INDEX stored_document_events_document_id_idx ON stored_document_events(document_id, created_at);
COMMIT;
