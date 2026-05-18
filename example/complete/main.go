package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nicolasmmb/envx"
)

type Config struct {
	Name string `envx:"name=NAME,default=envx-complete"`
	Env  string `envx:"name=ENV,default=local,enum=\"local,staging,production\""`
	Port int    `envx:"name=PORT,default=8080"`

	Database struct {
		URL      string `envx:"name=URL,required=true,secret=true"`
		PoolSize int    `envx:"name=POOL_SIZE,default=10"`
	}

	Features struct {
		Debug          bool     `envx:"name=DEBUG,default=false"`
		AllowedOrigins []string `envx:"name=ALLOWED_ORIGINS,default=http://localhost:3000"`
	}

	ShutdownGrace time.Duration `envx:"name=SHUTDOWN_GRACE,default=10s"`
	LegacyToken   string        `envx:"name=LEGACY_TOKEN,deprecated=true"`
}

func main() {
	logger := log.New(os.Stdout, "[complete] ", log.LstdFlags)
	configPath := "example/complete/config.json"

	loader := envx.NewLoader[Config](
		envx.WithPrefix("APP"),
		envx.WithProvider(envx.File(configPath)),
		envx.WithProvider(envx.Map(map[string]string{
			"DATABASE_POOL_SIZE": "20",
		})),
		envx.WithPrecedence(envx.PrecedenceEnvWins),
		envx.WithValidator(func(cfg *Config) error {
			if cfg.Port < 1024 {
				return errors.New("APP_PORT must be >= 1024")
			}
			if cfg.Database.PoolSize < 1 {
				return errors.New("APP_DATABASE_POOL_SIZE must be >= 1")
			}
			return nil
		}),
		envx.WithOnReload(func(old *Config, new *Config) {
			logger.Printf("config reloaded: %d -> %d", old.Port, new.Port)
		}),
		envx.WithOnReloadError(func(err error) {
			logger.Printf("reload error: %v", err)
		}),
		envx.WithWatch(configPath, 2*time.Second),
	)

	cfg, err := loader.Load()
	if err != nil {
		logger.Fatalf("config load failed: %v", err)
	}
	envx.Print(cfg)

	if err := loader.StartWatching(); err != nil {
		logger.Fatalf("failed to watch config: %v", err)
	}
	defer loader.StopWatching()

	logger.Printf("running %s on :%d (env=%s)", cfg.Name, cfg.Port, cfg.Env)
	logger.Printf("edit %s to trigger hot reload", configPath)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	<-sig
	fmt.Println("\nshutting down")
}
