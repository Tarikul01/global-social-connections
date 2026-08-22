-- Profile enrichment: pitch, quote, views counter
ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS pitch_url TEXT,
    ADD COLUMN IF NOT EXISTS personal_quote TEXT,
    ADD COLUMN IF NOT EXISTS profile_views_count INT NOT NULL DEFAULT 0;

-- Optional detailed view log (for future analytics)
CREATE TABLE IF NOT EXISTS profile_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    viewer_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_profile_views_profile ON profile_views(profile_user_id, created_at DESC);
