-- Ensure map-visible sharing for demo users and local dev account
UPDATE location_settings SET sharing_mode = 'discoverable', discovery_enabled = true, show_on_map = true
WHERE user_id IN (
  SELECT id FROM users WHERE username = 'tarikul420' OR email LIKE '%@demo.findneighbour.app'
);

UPDATE user_locations SET city = 'Dholairpar', region = 'Dhaka Division', country = 'Bangladesh'
WHERE user_id = (SELECT id FROM users WHERE username = 'tarikul420');
