module github.com/katheland/chirpy

go 1.24.1

require internal/database v0.0.0

require internal/auth v0.0.0

require (
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/lib/pq v1.10.9 // indirect
	golang.org/x/crypto v0.36.0 // indirect
)

replace internal/database => ./internal/database

replace internal/auth => ./internal/auth
