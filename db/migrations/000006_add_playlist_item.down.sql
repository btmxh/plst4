ALTER TABLE playlists DROP CONSTRAINT fk_current;
ALTER TABLE playlists DROP COLUMN current;
ALTER TABLE playlists DROP COLUMN current_idx;
DROP TABLE IF EXISTS playlist_items;
DROP TABLE IF EXISTS medias;
