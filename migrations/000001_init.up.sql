CREATE TABLE customers (
	id UUID PRIMARY KEY DEFAULT uuidv7(),
	name VARCHAR(255) NOT NULL CHECK (btrim(name) <> ''),
	email VARCHAR(255) NOT NULL UNIQUE CHECK (btrim(email) <> ''),
	phone VARCHAR(30) NOT NULL CHECK (btrim(phone) <> ''),
	password_hash VARCHAR(60) NOT NULL CHECK (btrim(password_hash) <> ''),
	role VARCHAR(20) NOT NULL DEFAULT 'customer'
		CHECK (role IN ('customer', 'admin')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE refresh_token_families (
	id UUID PRIMARY KEY,
	customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CHECK (expires_at > created_at),
	CHECK (revoked_at IS NULL OR revoked_at >= created_at)
);

CREATE INDEX refresh_token_families_customer_id_idx
	ON refresh_token_families (customer_id);

CREATE INDEX refresh_token_families_expires_at_idx
	ON refresh_token_families (expires_at);

CREATE TABLE refresh_tokens (
	id UUID PRIMARY KEY,
	family_id UUID NOT NULL REFERENCES refresh_token_families(id) ON DELETE CASCADE,
	token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	replaced_by UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	CHECK (expires_at > created_at),
	CHECK (revoked_at IS NULL OR revoked_at >= created_at),
	CHECK (replaced_by IS NULL OR replaced_by <> id)
);

CREATE INDEX refresh_tokens_family_id_idx
	ON refresh_tokens (family_id);

CREATE INDEX refresh_tokens_expires_at_idx
	ON refresh_tokens (expires_at);

-- Uma família representa uma sessão. Durante a rotação, o token anterior é
-- revogado antes da inserção do substituto, portanto só pode existir um token
-- ativo por família.
CREATE UNIQUE INDEX refresh_tokens_one_active_per_family_idx
	ON refresh_tokens (family_id)
	WHERE revoked_at IS NULL;
