-- Go-owned auth schema. No external migration history or bootstrap data.
BEGIN;
CREATE TABLE epcerp_schema_identity (
 singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK(singleton),
 project TEXT NOT NULL CHECK(project = 'epcERP')
);
INSERT INTO epcerp_schema_identity(singleton, project) VALUES(true, 'epcERP');
CREATE TABLE users (
	id UUID NOT NULL, 
	username VARCHAR(64) NOT NULL, 
	email VARCHAR(255) NOT NULL, 
	password_hash VARCHAR(255) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	is_superuser BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE roles (
	id UUID NOT NULL, 
	name VARCHAR(64) NOT NULL, 
	description VARCHAR(255) NOT NULL, 
	is_active BOOLEAN NOT NULL, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	created_by_id UUID, 
	updated_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	updated_by_id UUID, 
	PRIMARY KEY (id), 
	UNIQUE (name), 
	FOREIGN KEY(created_by_id) REFERENCES users (id) ON DELETE SET NULL, 
	FOREIGN KEY(updated_by_id) REFERENCES users (id) ON DELETE SET NULL
)

;

CREATE TABLE permissions (
	id UUID NOT NULL, 
	code VARCHAR(128) NOT NULL, 
	description VARCHAR(255) NOT NULL, 
	PRIMARY KEY (id), 
	UNIQUE (code)
)

;

CREATE TABLE user_roles (
	user_id UUID NOT NULL, 
	role_id UUID NOT NULL, 
	PRIMARY KEY (user_id, role_id), 
	FOREIGN KEY(user_id) REFERENCES users (id) ON DELETE CASCADE, 
	FOREIGN KEY(role_id) REFERENCES roles (id) ON DELETE CASCADE
)

;

CREATE TABLE role_permissions (
	role_id UUID NOT NULL, 
	permission_id UUID NOT NULL, 
	PRIMARY KEY (role_id, permission_id), 
	FOREIGN KEY(role_id) REFERENCES roles (id) ON DELETE CASCADE, 
	FOREIGN KEY(permission_id) REFERENCES permissions (id) ON DELETE CASCADE
)

;

CREATE TABLE refresh_tokens (
	id UUID NOT NULL, 
	user_id UUID NOT NULL, 
	token_hash VARCHAR(64) NOT NULL, 
	family_id UUID NOT NULL, 
	expires_at TIMESTAMP WITH TIME ZONE NOT NULL, 
	revoked_at TIMESTAMP WITH TIME ZONE, 
	created_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	PRIMARY KEY (id), 
	CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash), 
	FOREIGN KEY(user_id) REFERENCES users (id) ON DELETE CASCADE
)

;

CREATE TABLE user_activity (
	user_id UUID NOT NULL, 
	current_page VARCHAR(255) NOT NULL, 
	last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT now() NOT NULL, 
	PRIMARY KEY (user_id), 
	FOREIGN KEY(user_id) REFERENCES users (id) ON DELETE CASCADE
)

;
CREATE UNIQUE INDEX users_username_key ON users(username);
CREATE UNIQUE INDEX users_email_key ON users(email);
CREATE INDEX user_roles_role_id_idx ON user_roles(role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions(permission_id);
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens(user_id);
CREATE INDEX refresh_tokens_family_id_idx ON refresh_tokens(family_id);
COMMIT;
