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
	"database/sql"
	"os"
	"time"
	"github.com/joho/godotenv"
	"github.com/google/uuid"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries *database.Queries
	platform string
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/metrics", func(wri http.ResponseWriter, req *http.Request){
		respondWithString(wri, 200, fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", apiCfg.fileserverHits.Load()))
	})
	mux.HandleFunc("POST /admin/reset", func(wri http.ResponseWriter, req *http.Request){
		if apiCfg.platform == "dev" {
			apiCfg.metricsReset()
			apiCfg.dbQueries.ResetUsers(req.Context())
			respondWithString(wri, 200, "Reset")
		} else {
			respondWithError(wri, 403, "Forbidden")
		}
	})

	mux.HandleFunc("GET /api/healthz", func(wri http.ResponseWriter, req *http.Request) {
		respondWithString(wri, 200, "OK")
	})
	mux.HandleFunc("POST /api/validate_chirp", func(wri http.ResponseWriter, req *http.Request) {
		type reqParam struct {
			Body string `json:"body"`
		}
		type resParam struct {
			CleanedBody string `json:"cleaned_body"`
		}
		
		// first decode the request
		decoder := json.NewDecoder(req.Body)
		reqBody := reqParam{}
		err := decoder.Decode(&reqBody)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprint("Error decoding request: %v", err))
			return
		}

		// then set up the response
		if len(reqBody.Body) > 140 {
			respondWithError(wri, 400, "Chirp is too long")
			return
		}
		resBody := resParam{CleanedBody: profanityFilter(reqBody.Body)}
		respondWithJSON(wri, 200, resBody)
	})
	mux.HandleFunc("POST /api/users", func(wri http.ResponseWriter, req *http.Request) {
		type reqParam struct {
			Email string `json:"email"`
		}
		type resParam struct {
			ID uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Email string `json:"email"`
		}

		// first decode the request
		decoder := json.NewDecoder(req.Body)
		reqBody := reqParam{}
		err := decoder.Decode(&reqBody)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprint("Error decoding request: %v", err))
			return
		}

		user, err := apiCfg.dbQueries.CreateUser(req.Context(), reqBody.Email)
		if err != nil {
			respondWithError(wri, 500, fmt.Sprint("Error creating user: %v", err))
			return
		}
		resBody := resParam{
			ID: user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.CreatedAt,
			Email: user.Email,
		}
		respondWithJSON(wri, 201, resBody)
	})

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