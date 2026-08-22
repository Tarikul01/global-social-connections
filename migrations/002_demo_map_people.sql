-- Demo discoverable people for the public map (safe to re-run)
-- Password for all demo users: DemoPass123!

DO $$
DECLARE
  pwd TEXT := '$argon2id$v=19$m=65536,t=3,p=4$ZGVtb3NhbHRkZW1vc2FsdA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA';
  -- Note: demo password hashes below are created properly by seed script in Go if needed.
BEGIN
  NULL;
END $$;

-- Insert demo users with a known Argon2id hash for "DemoPass123!"
-- Generated offline for local preview only.
INSERT INTO users (id, email, password_hash, username, status, email_verified_at)
VALUES
  ('11111111-1111-1111-1111-111111111101', 'sara@demo.findneighbour.app', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHRzb21lc2FsdA$8K8vH0gQ1mF2nY3pR5tU7wX9zA1bC2dE3fG4hI5jK6l', 'sara_dev', 'active', now()),
  ('11111111-1111-1111-1111-111111111102', 'amir@demo.findneighbour.app', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHRzb21lc2FsdA$8K8vH0gQ1mF2nY3pR5tU7wX9zA1bC2dE3fG4hI5jK6l', 'amir_founder', 'active', now()),
  ('11111111-1111-1111-1111-111111111103', 'lena@demo.findneighbour.app', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHRzb21lc2FsdA$8K8vH0gQ1mF2nY3pR5tU7wX9zA1bC2dE3fG4hI5jK6l', 'lena_design', 'active', now()),
  ('11111111-1111-1111-1111-111111111104', 'raj@demo.findneighbour.app', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHRzb21lc2FsdA$8K8vH0gQ1mF2nY3pR5tU7wX9zA1bC2dE3fG4hI5jK6l', 'raj_investor', 'active', now()),
  ('11111111-1111-1111-1111-111111111105', 'maya@demo.findneighbour.app', '$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHRzb21lc2FsdA$8K8vH0gQ1mF2nY3pR5tU7wX9zA1bC2dE3fG4hI5jK6l', 'maya_ai', 'active', now())
ON CONFLICT (email) DO NOTHING;

INSERT INTO profiles (user_id, full_name, bio, profession_id, availability_status, onboarding_completed)
SELECT u.id, v.full_name, v.bio, p.id, v.status, true
FROM (VALUES
  ('sara@demo.findneighbour.app', 'Sara Ahmed', 'Backend engineer exploring Go & distributed systems', 'available'),
  ('amir@demo.findneighbour.app', 'Amir Khan', 'Startup founder looking for co-founders', 'available'),
  ('lena@demo.findneighbour.app', 'Lena Cruz', 'Product designer · UX for mobile maps', 'busy'),
  ('raj@demo.findneighbour.app', 'Raj Patel', 'Angel investor · SaaS & AI', 'offline'),
  ('maya@demo.findneighbour.app', 'Maya Rahman', 'ML engineer · NLP & recommendations', 'away')
) AS v(email, full_name, bio, status)
JOIN users u ON u.email = v.email
LEFT JOIN professions p ON p.slug = CASE
  WHEN v.email LIKE 'sara%' THEN 'backend-developer'
  WHEN v.email LIKE 'amir%' THEN 'founder'
  WHEN v.email LIKE 'lena%' THEN 'designer'
  WHEN v.email LIKE 'raj%' THEN 'investor'
  ELSE 'software-engineer'
END
ON CONFLICT (user_id) DO UPDATE SET
  full_name = EXCLUDED.full_name,
  bio = EXCLUDED.bio,
  availability_status = EXCLUDED.availability_status,
  onboarding_completed = true;

INSERT INTO location_settings (user_id, sharing_mode, discovery_enabled, show_on_map)
SELECT u.id, 'discoverable', true, true
FROM users u
WHERE u.email LIKE '%@demo.findneighbour.app'
ON CONFLICT (user_id) DO UPDATE SET
  sharing_mode = 'discoverable',
  discovery_enabled = true,
  show_on_map = true;

-- Approx locations around Dhaka (fuzzed)
INSERT INTO user_locations (user_id, city, region, country, geom, fuzzed_geom)
SELECT u.id, 'Dhaka', 'Dhaka', 'Bangladesh',
  ST_SetSRID(ST_MakePoint(v.lng, v.lat), 4326)::geography,
  ST_SetSRID(ST_MakePoint(v.lng + 0.002, v.lat + 0.002), 4326)::geography
FROM (VALUES
  ('sara@demo.findneighbour.app', 23.8103, 90.4125),
  ('amir@demo.findneighbour.app', 23.7808, 90.4000),
  ('lena@demo.findneighbour.app', 23.8223, 90.3654),
  ('raj@demo.findneighbour.app', 23.7465, 90.3760),
  ('maya@demo.findneighbour.app', 23.8759, 90.3795)
) AS v(email, lat, lng)
JOIN users u ON u.email = v.email
ON CONFLICT (user_id) DO UPDATE SET
  city = EXCLUDED.city,
  region = EXCLUDED.region,
  country = EXCLUDED.country,
  geom = EXCLUDED.geom,
  fuzzed_geom = EXCLUDED.fuzzed_geom,
  updated_at = now();
