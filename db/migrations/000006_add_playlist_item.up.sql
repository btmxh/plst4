CREATE TABLE IF NOT EXISTS medias (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title VARCHAR(255) NOT NULL,
  artist VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS playlist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  media UUID REFERENCES medias(id),
  prev UUID REFERENCES playlist_items(id),
  next UUID REFERENCES playlist_items(id),
  playlist UUID REFERENCES playlists(id),
  add_timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE playlists ADD COLUMN current_idx INT DEFAULT -1;
ALTER TABLE playlists ADD COLUMN current UUID;
ALTER TABLE playlists ADD CONSTRAINT fk_current FOREIGN KEY (current) REFERENCES playlist_items(id);
ALTER TABLE playlists ADD COLUMN first UUID;
ALTER TABLE playlists ADD CONSTRAINT fk_first FOREIGN KEY (first) REFERENCES playlist_items(id);
ALTER TABLE playlists ADD COLUMN last UUID;
ALTER TABLE playlists ADD CONSTRAINT fk_last FOREIGN KEY (last) REFERENCES playlist_items(id);
