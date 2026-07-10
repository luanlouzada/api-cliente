CREATE TABLE IF NOT EXISTS refresh_tokens (
	id UUID PRIMARY KEY,
	customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
	family_id UUID NOT NULL,
	token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
	expires_at TIMESTAMPTZ NOT NULL,
	revoked_at TIMESTAMPTZ,
	replaced_by UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS refresh_tokens_customer_id_idx
	ON refresh_tokens (customer_id);

CREATE INDEX IF NOT EXISTS refresh_tokens_family_id_idx
	ON refresh_tokens (family_id);

CREATE INDEX IF NOT EXISTS refresh_tokens_expires_at_idx
	ON refresh_tokens (expires_at);
