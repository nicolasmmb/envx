// Example demonstrates basic usage of envconfig
package main

import (
	"fmt"
	"time"

	"github.com/nicolasmmb/envx"
)

type Config struct {
	// Server
	Port int    `default:"8080"`
	Host string `default:"0.0.0.0"`

	// Database
	DatabaseURL string `required:"true" secret:"true"`

	// Features
	Debug   bool          `default:"false"`
	Timeout time.Duration `default:"30s"`

	// Lists
	AllowedOrigins []string `default:"http://localhost:3000"`
}

func main() {
	// Simple usage - loads from defaults + environment
	cfg, err := envx.Load[Config]()
	if err != nil {
		panic(err)
	}

	// Print configuration (secrets are masked)
	envx.Print(cfg)

	fmt.Printf("\nServer starting on %s:%d\n", cfg.Host, cfg.Port)
}
