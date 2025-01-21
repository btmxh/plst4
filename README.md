# plst4

Go rewrite of [plst3](https://github.com/btmxh/plst3).

## Usage

plst4 aims to unite all media platform under one common interface. Create a
playlist and add media to it. Do watchalongs with others. No need to worry about
storage space, nothing is downloaded under the hood, as HTML5 embed players are
used for playback.

Heavily based on [cytube](https://github.com/calzoneman/sync).

## Deployment guide

- Create a PostgreSQL database and apply all available migration scripts with
  the [golang-migrate](https://github.com/golang-migrate/migrate) tool.
  ```sh
  go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
  export DATABASE_URL=postgresql://plst4:plst4@localhost:5432/plst4?sslmode=disable
  ~/go/bin/migrate -database "$DATABASE_URL" -path db/migrations up
  ```
- Install NPM dependencies
  ```sh
  npm install
  ```
- Set up the .env config file: create a file named .env in the root directory
  with the following content (these values are here for demonstration purposes,
  you should change them according to your needs):
  ```env
  DATABASE_URL=postgresql://plst4:plst4@localhost:5432/plst4?sslmode=disable
  PLST4_ADDR=0.0.0.0:443 # or 0.0.0.0:80 if HTTP-only
  MAIL_MODE=netmail
  # this example uses Gmail SMTP to setup email, consult the documentation of
  # your email provider for the exact instructions
  MAIL_HOST=smtp.gmail.com
  MAIL_PORT=587
  MAIL_EMAIL=plst@gmail.com
  MAIL_PASSWORD=secret
  # HTTPS configuration, leave empty for HTTP-only
  HTTPS_CERT_FILE=cert.pem
  HTTPS_KEY_FILE=key.pem
  # JWT secret key (change this!)
  JWT_SECRET=secret
  ```
- Build and run the application
  ```sh
  npm run build-go # build Go binary
  npm run build-scss && npm run build-ts # build web assets
  sudo ./plst4 # root privileges are required since we are using port 443/80
  ```

## License

This project is released under the Affero General Public License v3.
