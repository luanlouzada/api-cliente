ALTER TABLE refresh_tokens
	ADD COLUMN family_expires_at TIMESTAMPTZ;

UPDATE refresh_tokens
SET family_expires_at = expires_at
WHERE family_expires_at IS NULL;

ALTER TABLE refresh_tokens
	ALTER COLUMN family_expires_at SET NOT NULL,
	ADD CONSTRAINT refresh_tokens_expiration_order_check
		CHECK (expires_at <= family_expires_at);

CREATE INDEX refresh_tokens_family_expires_at_idx
	ON refresh_tokens (family_expires_at);
