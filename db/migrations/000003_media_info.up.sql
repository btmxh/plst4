ALTER TABLE medias ADD COLUMN media_type VARCHAR(10);
ALTER TABLE medias ADD COLUMN aspect_ratio VARCHAR(20) DEFAULT '16/9';
UPDATE medias SET media_type = 'yt';
ALTER TABLE medias ALTER COLUMN media_type SET NOT NULL;

CREATE TABLE IF NOT EXISTS alt_metadata(
  playlist INT NOT NULL REFERENCES playlists(id),
  media INT NOT NULL REFERENCES medias(id),
  alt_title VARCHAR(255),
  alt_artist VARCHAR(255),
  PRIMARY KEY (playlist, media)
);
