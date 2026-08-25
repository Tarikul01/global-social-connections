-- Event invitations (host invites users; invitee accepts to join)
CREATE TABLE IF NOT EXISTS event_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    inviter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    UNIQUE (event_id, invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_event_invitations_event ON event_invitations(event_id);
CREATE INDEX IF NOT EXISTS idx_event_invitations_invitee ON event_invitations(invitee_id);

-- Backfill hosts as members for events created before this migration
INSERT INTO event_members (event_id, user_id, role)
SELECT e.id, e.creator_id, 'host'
FROM events e
WHERE NOT EXISTS (
    SELECT 1 FROM event_members em WHERE em.event_id = e.id AND em.user_id = e.creator_id
)
ON CONFLICT (event_id, user_id) DO NOTHING;
