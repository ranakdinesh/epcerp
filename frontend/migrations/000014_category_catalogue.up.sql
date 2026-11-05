BEGIN;
CREATE TABLE categories (
id uuid PRIMARY KEY, code varchar(32) NOT NULL UNIQUE, name varchar(255) NOT NULL,
 is_active boolean NOT NULL DEFAULT true, version integer NOT NULL DEFAULT 1 CHECK(version > 0),
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 legacy_source jsonb, source_fingerprint text
);
CREATE TABLE catalogue_units (
id uuid PRIMARY KEY, code varchar(16) NOT NULL UNIQUE, name varchar(128) NOT NULL,
 is_active boolean NOT NULL DEFAULT true, version integer NOT NULL DEFAULT 1 CHECK(version > 0),
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 legacy_source jsonb, source_fingerprint text
);
CREATE TABLE category_subcategories (
id uuid PRIMARY KEY, code varchar(32) NOT NULL UNIQUE, name varchar(255) NOT NULL, category_id uuid NOT NULL REFERENCES categories(id), unit_id uuid NOT NULL REFERENCES catalogue_units(id), configuration jsonb NOT NULL,
 is_active boolean NOT NULL DEFAULT true, version integer NOT NULL DEFAULT 1 CHECK(version > 0),
 created_at timestamptz NOT NULL DEFAULT now(), created_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 updated_at timestamptz NOT NULL DEFAULT now(), updated_by_id uuid REFERENCES users(id) ON DELETE SET NULL,
 legacy_source jsonb, source_fingerprint text
);
CREATE INDEX category_subcategories_parent ON category_subcategories(category_id);
COMMIT;
