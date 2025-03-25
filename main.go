package main

import _ "github.com/lib/pq"

import (
	"fmt"
	"sync/atomic"
	"net/http"
	"encoding/json"
	"strings"
	"slices"
	"internal/database"
	"internal/auth"
	"database/sql"
	"os"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
	"time"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	platform string
	secret string
}

func main() {
	godotenv.Load() // loads the .env file
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("Error opening the database")
	}
	dbQueries := database.New(db)

	apiCfg := apiConfig{}
	apiCfg.fileserverHits.Store(0)
	apiCfg.dbQueries = dbQueries
	apiCfg.platform = os.Getenv("PLATFORM")
	apiCfg.secret = os.Getenv("SECRET")
	mux := http.NewServeMux()
	// get number of page visits
	mux.HandleFunc("GET /admin/metrics", func(wri http.ResponseWriter, req *http.Request){
		respondWithString(wri, 200, fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", apiCfg.fileserverHits.Load()))
	})
	// a dev endpoint to reset page visits and the database
	mux.HandleFunc("POST /admin/reset", func(wri http.ResponseWriter, req *http.Request){
		if apiCfg.platform == "dev" {
			apiCfg.metricsReset()
			apiCfg.dbQueries.ResetUsers(req.Context())
			respondWithString(wri, 200, "Reset")
		} else {
			respondWithError(wri, 403, "Forbidden")
		}
	})
	// get the health of the server
	mux.HandleFunc("GET /api/healthz", func(wri http.ResponseWriter, req *http.Request) {
		respondWithString(wri, 200, "OK")
	})
	// get all chirps
	mux.HandleFunc("GET /api/chirps", func(wri http.ResponseWriter, req *http.Request) {
		chirps, err := apiCfg.dbQueries.GetAllChirps(req.Context())
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting chirps: %v", err))
			return
		}
		output := []chirpParam{}
		for _, c := range chirps {
			output = append(output, chirpParam{
				ID: c.ID,
				CreatedAt: c.CreatedAt,
				UpdatedAt: c.UpdatedAt,
				Body: c.Body,
				UserID: c.UserID,
			})
		}
		respondWithJSON(wri, 200, output)
	})
	// get chirp by ID
	mux.HandleFunc("GET /api/chirps/{chirpID}", func(wri http.ResponseWriter, req *http.Request) {
		chirpID, _ := uuid.Parse(req.PathValue("chirpID"))
		chirp, err := apiCfg.dbQueries.GetSingleChirp(req.Context(), chirpID)
		if err != nil {
			if strings.Contains(fmt.Sprint(err), "no rows in result set") {
				respondWithError(wri, 404, fmt.Sprint("Chirp not found"))
			} else {
				respondWithError(wri, 500, fmt.Sprintf("Error getting chirp: %v", err))
			}
			return
		}
		resBody := chirpParam {
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
		}
		respondWithJSON(wri, 200, resBody)
	})
	// create a new chirp
	mux.HandleFunc("POST /api/chirps", func(wri http.ResponseWriter, req *http.Request) {
		type reqParam struct {
			Body string `json:"body"`
			UserID uuid.UUID `json:"user_id"`
		}
		
		// first decode the request
		decoder := json.NewDecoder(req.Body)
		reqBody := reqParam{}
		err := decoder.Decode(&reqBody)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
			return
		}
		if len(reqBody.Body) > 140 {
			respondWithError(wri, 400, "Chirp is too long")
			return
		}

		// make sure the user is valid
		bearer, err := auth.GetBearerToken(req.Header)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
			return
		}
		user, err := auth.ValidateJWT(bearer, apiCfg.secret)
		if err != nil {
			respondWithError(wri, 401, "Unauthorized")
			return
		}
		
		chirp, err := apiCfg.dbQueries.CreateChirp(req.Context(), database.CreateChirpParams{Body: profanityFilter(reqBody.Body), UserID: user,})
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error creating chirp: %v", err))
			return
		}
		resBody := chirpParam{
			ID: chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.CreatedAt,
			Body: chirp.Body,
			UserID: chirp.UserID,
		}
		respondWithJSON(wri, 201, resBody)
	})
	// create a new user
	mux.HandleFunc("POST /api/users", func(wri http.ResponseWriter, req *http.Request) {
		type reqParam struct {
			Email string `json:"email"`
			Password string `json:"password"`
		}
		
		// first decode the request
		decoder := json.NewDecoder(req.Body)
		reqBody := reqParam{}
		err := decoder.Decode(&reqBody)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
			return
		}

		hashword, err := auth.HashPassword(reqBody.Password)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error hashing password: %v", err))
		}
		user, err := apiCfg.dbQueries.CreateUser(req.Context(), database.CreateUserParams{Email: reqBody.Email, HashedPassword: hashword})
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error creating user: %v", err))
			return
		}
		resBody := userParam{
			ID: user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.CreatedAt,
			Email: user.Email,
		}
		respondWithJSON(wri, 201, resBody)
	})
	// login
	mux.HandleFunc("POST /api/login", func(wri http.ResponseWriter, req *http.Request) {
		type reqParam struct {
			Email string `json:"email"`
			Password string `json:"password"`
		}

		// first decode the request
		decoder := json.NewDecoder(req.Body)
		reqBody := reqParam{}
		err := decoder.Decode(&reqBody)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error decoding request: %v", err))
			return
		}

		// check authorization
		user, err := apiCfg.dbQueries.GetUserByEmail(req.Context(), reqBody.Email)
		if err != nil {
			respondWithError(wri, 401, "Incorrect username or password")
		}
		err = auth.CheckPasswordHash(reqBody.Password, user.HashedPassword)
		if err != nil {
			respondWithError(wri, 401, "Incorrect username or password")
		}

		// get jwt token
		dura, _ := time.ParseDuration(fmt.Sprintf("3600s"))
		jwtToken, err := auth.MakeJWT(user.ID, apiCfg.secret, dura)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting JWT token: %v", err))
		}
		tokenStr, err := auth.MakeRefreshToken()
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("I'm surprised. %v", err))
		}
		refDura, _ := time.ParseDuration(fmt.Sprintf("1440h"))
		refreshToken, err := apiCfg.dbQueries.CreateToken(req.Context(), database.CreateTokenParams{
			Token: tokenStr,
			UserID: user.ID,
			ExpiresAt: sql.NullTime{Time: time.Now().Add(refDura), Valid: true},
		})
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting refresh token: %v", err))
		}
		
		resBody := userParam{
			ID: user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.CreatedAt,
			Email: user.Email,
			Token: jwtToken,
			RefreshToken: refreshToken.Token,
		}
		respondWithJSON(wri, 200, resBody)
	})
	// get refreshed jwt token
	mux.HandleFunc("POST /api/refresh", func(wri http.ResponseWriter, req *http.Request) {
		// `the json:"token"` bit is essential, yes even more essential than that x_x
		type resParam struct {
			Token string `json:"token"` 
		}
		bearer, err := auth.GetBearerToken(req.Header)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
			return
		}
		userWithExpiration, err := apiCfg.dbQueries.GetUserFromRefreshToken(req.Context(), bearer)
		if err != nil {
			respondWithError(wri, 401, fmt.Sprintf("Unauthorized: %v", err))
			return
		}
		if userWithExpiration.RevokedAt.Valid == true {
			respondWithError(wri, 401, "Revoked")
			return
		}
		now := time.Now()
		if userWithExpiration.ExpiresAt.Time.Before(now) {
			respondWithError(wri, 401, "Expired")
			return
		}
		
		dura, _ := time.ParseDuration(fmt.Sprintf("3600s"))
		jwtToken, err := auth.MakeJWT(userWithExpiration.UserID, apiCfg.secret, dura)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting JWT token: %v", err))
		}
		resBody := resParam{
			Token: jwtToken,
		}
		respondWithJSON(wri, 200, resBody)
	})
	mux.HandleFunc("POST /api/revoke", func(wri http.ResponseWriter, req *http.Request) {
		bearer, err := auth.GetBearerToken(req.Header)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error getting bearer token: %v", err))
			return
		}
		err = apiCfg.dbQueries.RevokeToken(req.Context(), bearer)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprintf("Error revoking token: %v", err))
			return
		}
		wri.WriteHeader(204)
	})

	// access a page on the website
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	_ = server.ListenAndServe()
}

// adds one to the metrics counter every time something on /app/ is accessed
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wri http.ResponseWriter, res *http.Request) {
		cfg.fileserverHits.Store(cfg.fileserverHits.Add(1))
		next.ServeHTTP(wri, res) // ALWAYS continue the ServeHTTP chain.  Don't just send the plain Handler.
	})
}

// resets the metrics counter to 0
func (cfg *apiConfig) metricsReset() {
	cfg.fileserverHits.Store(0)
}

// cleans a string to replace disallowed words with asterisks
func profanityFilter(original string) string {
	disallowed := []string{"kerfuffle", "sharbert", "fornax"} //I'm just doing what the lesson says.
	tokenified := strings.Split(original, " ")
	for i, _ := range tokenified {
		if slices.Contains(disallowed, strings.ToLower(tokenified[i])) {
			tokenified[i] = "****"
		}
	}
	return strings.Join(tokenified, " ")
}