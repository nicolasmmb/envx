package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nicolasmmb/envx"
)

type Config struct {
	Port int    `default:"8080"`
	Host string `default:"0.0.0.0"`

	DatabaseURL string `required:"true" secret:"true"`

	Debug   bool          `default:"false"`
	Timeout time.Duration `default:"30s"`

	AllowedOrigins []string `default:"http://localhost:3000"`
}

func main() {
	cfg, err := envx.Load[Config](
		envx.WithProvider(envx.File(".env")), // optional local overrides
	)
	if err != nil {
		log.Fatal(err)
	}

	envx.Print(cfg)
	fmt.Printf("\nServer starting on %s:%d\n", cfg.Host, cfg.Port)
}
