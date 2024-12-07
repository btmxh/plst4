ALTER TABLE playlists ADD COLUMN state_secret UUID DEFAULT gen_random_uuid();
