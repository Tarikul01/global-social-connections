-- Birthday & gender for profile card
ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS gender VARCHAR(30),
    ADD COLUMN IF NOT EXISTS birthday DATE;
