DROP INDEX IF EXISTS refresh_tokens_family_expires_at_idx;

ALTER TABLE refresh_tokens
	DROP CONSTRAINT IF EXISTS refresh_tokens_expiration_order_check,
	DROP COLUMN IF EXISTS family_expires_at;
