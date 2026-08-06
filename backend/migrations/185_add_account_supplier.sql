-- Add the supplier dimension used by account cost reporting.
-- Existing accounts remain valid and are grouped as "supplier not set" until labeled.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS supplier VARCHAR(100) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS account_supplier
    ON accounts (supplier);
