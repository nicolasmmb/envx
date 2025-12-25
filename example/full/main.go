package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nicolasmmb/envx"
)

// Config demonstrates most envx features in one place.
type Config struct {
	App struct {
		Name string `default:"envx"`
		Env  string `default:"local"`
		Port int    `default:"8080"`
	}

	Database struct {
		URL      string `required:"true" secret:"true"`
		MaxConns int    `default:"10"`
	}

	Features struct {
		Debug          bool     `default:"false"`
		AllowedOrigins []string `default:"http://localhost:3000"`
	}

	ShutdownGrace time.Duration `default:"10s"`
}

func main() {
	loader := envx.NewLoader[Config](
		envx.WithPrefix("APP"),                      // strict prefix
		envx.WithProvider(envx.Defaults[Config]()),  // defaults (auto-prefixed)
		envx.WithProvider(envx.File("config.json")), // optional JSON/.env file
		envx.WithProvider(envx.Env()),               // environment
		envx.WithValidator(func(cfg *Config) error { // type-safe validator
			if cfg.App.Port < 1024 {
				return errors.New("APP_PORT must be >= 1024")
			}
			return nil
		}),
		envx.WithOnReload(func(old *Config, new *Config) {
			log.Printf("config reloaded: port %d -> %d", old.App.Port, new.App.Port)
		}),
		envx.WithWatch("config.json", 2*time.Second), // hot reload
	)

	cfg := loader.MustLoad()
	envx.Print(cfg)

	if err := loader.StartWatching(); err != nil {
		log.Fatalf("failed to start watcher: %v", err)
	}
	defer loader.StopWatching()

	fmt.Printf("\nRunning %s on :%d (env=%s)\n", cfg.App.Name, cfg.App.Port, cfg.App.Env)
	select {}
}
