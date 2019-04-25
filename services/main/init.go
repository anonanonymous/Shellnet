package main

import (
	"html/template"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/middleware/stdlib"
	"github.com/ulule/limiter/drivers/store/memory"

	"github.com/gomodule/redigo/redis"
)

var (
	hostURI, hostPort     string
	usrURI                string
	walletURI             string
	sessionDB             *redis.Pool
	ratelimiter, strictRL *stdlib.Middleware
	templates             *template.Template
	logFile               *os.File
)

func init() {
	var err error

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = ":6379"
	}
	if hostURI = os.Getenv("HOST_URI"); hostURI == "" {
		panic("Set the HOST_URI env variable")
	}
	if hostPort = os.Getenv("HOST_PORT"); hostPort == "" {
		hostPort = ":8080"
		println("Using default HOST_PORT - 8080")
	}

	// Removed this in favor of local nginx routing to 8080.
	// To keep routing to specific port, include port in run.sh HOST_URI.
	hostURI += hostPort

	if usrURI = os.Getenv("USER_URI"); usrURI == "" {
		panic("Set the USER_URI env variable")
	}
	if walletURI = os.Getenv("WALLET_URI"); walletURI == "" {
		panic("Set the WALLET_URI env variable")
	}

	// logging setup
	logFile, err = os.OpenFile("service.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	// ratelimiter config
	ratelimiter = stdlib.NewMiddleware(limiter.New(memory.NewStore(), limiter.Rate{
		Period: time.Minute * 1,
		Limit:  100,
	}))
	strictRL = stdlib.NewMiddleware(limiter.New(memory.NewStore(), limiter.Rate{
		Period: time.Hour * 24,
		Limit:  10,
	}))

	templates = template.Must(template.ParseGlob("templates/*.html"))
	sessionDB = newPool(redisHost)
	cleanupHook()
}
