CREATE TABLE IF NOT EXISTS users(
  username VARCHAR(50) PRIMARY KEY,
  password_hashed CHAR(60) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  register_timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS pending_users(
  username VARCHAR(50) PRIMARY KEY,
  password_hashed CHAR(60) NOT NULL,
  email VARCHAR(255) UNIQUE NOT NULL,
  identifier CHAR(16) NOT NULL,
  expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '5 minutes')
);

CREATE TABLE IF NOT EXISTS password_reset(
  email VARCHAR(255) PRIMARY KEY,
  identifier CHAR(16) NOT NULL
);

CREATE TABLE IF NOT EXISTS playlists(
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  owner_username VARCHAR(50) NOT NULL,
  created_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
  current INT,
  FOREIGN KEY (owner_username) REFERENCES users(username)
);

CREATE TABLE IF NOT EXISTS medias (
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  artist VARCHAR(255) NOT NULL,
  url VARCHAR(255) NOT NULL UNIQUE,
  duration INT NOT NULL DEFAULT 0, -- in seconds
  add_timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS playlist_items (
  id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  media INT NOT NULL REFERENCES medias(id),
  playlist INT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
  item_order INT NOT NULL,
  add_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (playlist, item_order) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX idx_playlist_item_order ON playlist_items(playlist, item_order);
CREATE INDEX idx_media_url ON medias(url);
ALTER TABLE playlists ADD CONSTRAINT fk_current FOREIGN KEY (current) REFERENCES playlist_items(id);
