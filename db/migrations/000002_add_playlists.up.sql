CREATE TABLE IF NOT EXISTS playlists(
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  owner_username VARCHAR(50) NOT NULL,
  created_timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
  item_count INT DEFAULT 0,
  FOREIGN KEY (owner_username) REFERENCES users(username)
);
