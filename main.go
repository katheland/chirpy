package main

import (
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(wri http.ResponseWriter, req *http.Request) {
		wri.Header().Set("Content-Type", "text/plain; charset=utf-8")
		wri.WriteHeader(200)
		wri.Write([]byte("OK"))
	})

	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	
	server := http.Server{
		Handler: mux,
		Addr: ":8080",
	}
	_ = server.ListenAndServe()
}