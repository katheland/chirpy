# chirpy
An http server/toy chat api for Boot.dev

# Setup
You will need Go and Postgresql installed in order to run this program.

Go: `curl.exe https://webi.ms/golang | powershell`

Postgresql: `sudo apt install postgresql postgresql-contrib`

1. `sudo service postgresql start`
2. `sudo -u postgres psql`
3. `CREATE DATABASE chirpy;`

You will also likely want to install SQLC (`go install github.om/sqlc-dev/sqlc/cmd/sqlc@latest`) and run `sqlc generate` from the root of the project.

You will also need to create a .env file in the root of the project.  DB_URL should be the url of your postgres service, ex `postgres://postgres:@localhost:5432/chirpy`.  SECRET should be a randomly generated 64-bit string.

# Running Chirpy
The following endpoints are available and can be accessed through something like Postman.

- POST /api/users
Create a new user.  Request body is `{Email, Password}`
- POST /api/login
Log in to an existing user.  Request body is `{Email, Password}`
- PUT /api/
Changes the logged in user's email and password.  Requires a valid JWT token.  Request body is `{Email, Password}`

- GET /api/chirps?author_id=&sort=
Get all chirps.  If author_id is specified, get all chirps associated with that user.  Sort is either asc or desc, defaulting to asc.
- GET /api/chirps/{chirpID}
Get a single chirp by its ID.
- POST /api/chirps
Posts a new chirp.  Requires a valid JWT token.  Request body is `{Body, UserID}`
- DELETE /api/chirps/{chirpID}
Deletes a single chirp by its ID.  Requires a valid JWT token.

# Ideas For The Future
- I could actually have the web app use the api... that would probably be useful...
- Maybe filtering or searching the chirps?
- idk I don't use Twitter or Bluesky so idk what sorts of features would be useful