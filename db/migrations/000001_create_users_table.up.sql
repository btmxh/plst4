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
