package main

import (
	"html/template"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	"github.com/ulule/limiter/v3/drivers/store/memory"

	"github.com/gomodule/redigo/redis"
)

var (
	sessionDB             *redis.Pool
	ratelimiter, strictRL *stdlib.Middleware
	templates             *template.Template
	logFile               *os.File
)

const apiKEY = "dsanon"

func init() {
	var err error

	hostURI += hostPort

	// logging setup
	logFile, err = os.OpenFile(
		"service.log",
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		panic(err)
	}

	// ratelimiter config
	ratelimiter = stdlib.NewMiddleware(
		limiter.New(memory.NewStore(), limiter.Rate{
			Period: time.Minute * 1,
			Limit:  100,
		}),
	)
	strictRL = stdlib.NewMiddleware(
		limiter.New(memory.NewStore(), limiter.Rate{
			Period: time.Hour * 24,
			Limit:  100,
		}),
	)

	templates = template.Must(template.ParseGlob("templates/*.html"))
	sessionDB = newPool(redisHost)
	cleanupHook()
}
