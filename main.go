package main

import _ "github.com/lib/pq"

import (
	"fmt"
	"sync/atomic"
	"net/http"
	"strings"
	"slices"
	"internal/database"
	"database/sql"
	"os"
	"github.com/joho/godotenv"
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

	mux.HandleFunc("GET /api/chirps", func(wri http.ResponseWriter, req *http.Request) {
		getChirps(wri, req, apiCfg)
	})
	mux.HandleFunc("GET /api/chirps/{chirpID}", func(wri http.ResponseWriter, req *http.Request) {
		getChirpByID(wri, req, apiCfg)
	})
	mux.HandleFunc("POST /api/chirps", func(wri http.ResponseWriter, req *http.Request) {
		postChirp(wri, req, apiCfg)
	})
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", func(wri http.ResponseWriter, req *http.Request) {
		deleteChirp(wri, req, apiCfg)
	})
	
	mux.HandleFunc("POST /api/users", func(wri http.ResponseWriter, req *http.Request) {
		postUser(wri, req, apiCfg)
	})
	mux.HandleFunc("POST /api/login", func(wri http.ResponseWriter, req *http.Request) {
		postLogin(wri, req, apiCfg)
	})
	mux.HandleFunc("PUT /api/users", func(wri http.ResponseWriter, req *http.Request) {
		putUser(wri, req, apiCfg)
	})
	
	mux.HandleFunc("POST /api/refresh", func(wri http.ResponseWriter, req *http.Request) {
		refresh(wri, req, apiCfg)
	})
	mux.HandleFunc("POST /api/revoke", func(wri http.ResponseWriter, req *http.Request) {
		revoke(wri, req, apiCfg)
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