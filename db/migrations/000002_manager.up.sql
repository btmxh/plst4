CREATE TABLE IF NOT EXISTS playlist_manage(
  playlist INT NOT NULL,
  username VARCHAR(50) NOT NULL,
  PRIMARY KEY (playlist, username)
);
