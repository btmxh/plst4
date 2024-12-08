ALTER TABLE playlist_items
DROP CONSTRAINT playlist_items_playlist_fkey;

ALTER TABLE playlist_items
ADD CONSTRAINT playlist_items_playlist_fkey
FOREIGN KEY (playlist)
REFERENCES playlists(id);
