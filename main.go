package main

import (
	"fmt"
	"sync/atomic"
	"net/http"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	apiCfg := apiConfig{}
	apiCfg.fileserverHits.Store(0)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(wri http.ResponseWriter, req *http.Request) {
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte("OK"))
	})
	mux.HandleFunc("GET /metrics", func(wri http.ResponseWriter, req *http.Request){
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte(fmt.Sprintf("Hits: %v", apiCfg.fileserverHits.Load())))
	})
	mux.HandleFunc("POST /reset", func(wri http.ResponseWriter, req *http.Request){
		apiCfg.metricsReset()
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte(fmt.Sprintf("Reset! Hits: %v", apiCfg.fileserverHits.Load())))
	})

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	_ = server.ListenAndServe()
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wri http.ResponseWriter, res *http.Request) {
		cfg.fileserverHits.Store(cfg.fileserverHits.Add(1))
		next.ServeHTTP(wri, res) // ALWAYS continue the ServeHTTP chain.  Don't just send the plain Handler.
	})
}

func (cfg *apiConfig) metricsReset() {
	cfg.fileserverHits.Store(0)
}