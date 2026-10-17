package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nicolasmmb/envx"
)

// Environment is a native enum: envx validates values through EnumValues.
type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentStaging    Environment = "staging"
	EnvironmentProduction Environment = "production"
)

func (Environment) EnumValues() []string {
	return []string{
		string(EnvironmentLocal),
		string(EnvironmentStaging),
		string(EnvironmentProduction),
	}
}

// Config demonstrates most envx features in one place.
type Config struct {
	App struct {
		Name string      `envx:"name=NAME,default=envx"`
		Env  Environment `envx:"name=ENV,default=local"`
		Port int         `envx:"name=PORT,default=8080"`
	}

	Database struct {
		URL      string `envx:"name=URL,required=true,secret=true"`
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
		envx.WithProvider(envx.File("config.json")), // optional JSON/.env file
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

	cfg, err := loader.Load()
	if err != nil {
		logger.Fatalf("failed to load config: %v", err)
	}
	envx.Print(cfg)

	if err := loader.StartWatching(); err != nil {
		logger.Fatalf("failed to start watcher: %v", err)
	}
	defer loader.StopWatching()

	fmt.Printf("\nRunning %s on :%d (env=%s)\n", cfg.App.Name, cfg.App.Port, cfg.App.Env)
	select {}
}
