package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nicolasmmb/envx"
)

// Config demonstrates most envx features in one place.
type Config struct {
	App struct {
		Name string `envx:"name=NAME,default=envx"`
		Env  string `envx:"name=ENV,default=local"`
		Port int    `envx:"name=PORT,default=8080"`
	}

	Database struct {
		URL      string `envx:"name=DATABASE_URL,required=true,secret=true"`
		MaxConns int    `envx:"name=MAX_CONNS,default=10"`
	}

	Features struct {
		Debug          bool     `envx:"name=DEBUG,default=false"`
		AllowedOrigins []string `envx:"name=ALLOWED_ORIGINS,default=http://localhost:3000"`
	}

	ShutdownGrace time.Duration `envx:"name=SHUTDOWN_GRACE,default=10s"`
}

func main() {
	logger := log.New(os.Stdout, "[config] ", log.LstdFlags)

	loader := envx.NewLoader[Config](
		envx.WithLogger(logger),
		envx.WithPrefix("APP"),
		envx.WithProvider(envx.DefaultsWithPrefix[Config]("APP")),
		envx.WithProvider(envx.File("config.json")), // optional JSON/.env file
		envx.WithProvider(envx.Env()),               // environment
		envx.WithValidator(func(cfg *Config) error {
			if cfg.App.Port < 1024 {
				return errors.New("APP_PORT must be >= 1024")
			}
			return nil
		}),
		envx.WithOnReload(func(old *Config, new *Config) {
			logger.Printf("config reloaded: port %d -> %d", old.App.Port, new.App.Port)
		}),
		envx.WithWatch("config.json", 2*time.Second),
	)

	cfg := loader.MustLoad()
	envx.Print(cfg)

	if err := loader.StartWatching(); err != nil {
		logger.Fatalf("failed to start watcher: %v", err)
	}
	defer loader.StopWatching()

	fmt.Printf("\nRunning %s on :%d (env=%s)\n", cfg.App.Name, cfg.App.Port, cfg.App.Env)
	select {}
}
