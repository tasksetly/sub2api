-- 为上游后台会话保存 refresh token，使 access JWT 到期前可以无感续期。
ALTER TABLE upstream_providers
    ADD COLUMN IF NOT EXISTS refresh_token_encrypted TEXT;
