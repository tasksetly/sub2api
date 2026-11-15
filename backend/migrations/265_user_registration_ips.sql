-- Do not infer historical signup addresses from mutable login or usage records.
CREATE TABLE IF NOT EXISTS user_registration_ips (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    ip_address VARCHAR(45) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
