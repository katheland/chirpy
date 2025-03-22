package main

import (
	"fmt"
	"sync/atomic"
	"net/http"
	"encoding/json"
	"strings"
	"slices"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	apiCfg := apiConfig{}
	apiCfg.fileserverHits.Store(0)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", func(wri http.ResponseWriter, req *http.Request) {
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /admin/metrics", func(wri http.ResponseWriter, req *http.Request){
		wri.Header().Set("Content-Type", "text/html; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte(fmt.Sprintf("<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>", apiCfg.fileserverHits.Load())))
	})
	mux.HandleFunc("POST /admin/reset", func(wri http.ResponseWriter, req *http.Request){
		apiCfg.metricsReset()
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte(fmt.Sprintf("Reset! Hits: %v", apiCfg.fileserverHits.Load())))
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

// marshals the JSON data and sends it in response
func respondWithJSON(wri http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		respondWithError(wri, 500, fmt.Sprint("Error marshalling response: %v", err))
		return
	}
	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(code)
	wri.Write(dat)
}

// sends an error response
func respondWithError(wri http.ResponseWriter, code int, msg string) {
	type errorResp struct {
		Error string `json:"error"`
	}
	res := errorResp{Error: msg}
	ret, _ := json.Marshal(res)
	wri.Header().Set("Content-Type", "application/json")
	wri.WriteHeader(code)
	wri.Write(ret)
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