package main

import (
	"log"
	"net/http"
	"time"

	_ "github.com/lib/pq"

	"github.com/julienschmidt/httprouter"
)

func main() {
	defer logFile.Close()
	log.SetOutput(logFile)

	router := httprouter.New()

	srv := &http.Server{
		Addr:         hostPort,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  10 * time.Second,
		Handler:      router,
	}

	InitHandlers(router)

	/* https to http redirection
	go http.ListenAndServe(":80", http.HandlerFunc(httpsRedirect))
	log.Println("Info: Starting Service on:", hostURI)
	log.Fatal(srv.ListenAndServeTLS("fullchain.pem", "privkey.pem"))
	*/
	log.Println("Info: Starting Service on:", hostURI)
	log.Fatal(srv.ListenAndServe())
}
