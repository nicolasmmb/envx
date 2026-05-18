package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nicolasmmb/envx"
)

type Config struct {
	Port int    `envx:"name=PORT,default=8080"`
	Host string `envx:"name=HOST,default=0.0.0.0"`

	DatabaseURL string `envx:"name=DATABASE_URL,required=true,secret=true"`

	Debug   bool          `envx:"name=DEBUG,default=false"`
	Timeout time.Duration `envx:"name=TIMEOUT,default=30s"`

	AllowedOrigins []string `envx:"name=ALLOWED_ORIGINS,default=http://localhost:3000"`
}

func main() {
	cfg, err := envx.LoadFromEnv[Config]() // defaults + .env + env vars
	if err != nil {
		log.Fatal(err)
	}

	envx.Print(cfg)
	fmt.Printf("\nServer starting on %s:%d\n", cfg.Host, cfg.Port)
}
