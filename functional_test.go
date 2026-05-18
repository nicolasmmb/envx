package envx_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nicolasmmb/envx"
)

func chdirTemp(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if chErr := os.Chdir(oldwd); chErr != nil {
			t.Fatalf("restore cwd: %v", chErr)
		}
	})
	return dir
}

func TestFunctionalLoad_DefaultStack(t *testing.T) {
	chdirTemp(t)

	content := "PORT=5000\nAPP_NAME=dotenv-app\nFEATURES=metrics,tracing\nTIMEOUT=45s\nDB_URL=postgres://dotenv/service\nMODE=staging\n"
	if err := os.WriteFile(".env", []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("PORT", "7000")
	t.Setenv("MODE", "production")

	type Config struct {
		Port     int           `envx:"name=PORT,default=8080"`
		AppName  string        `envx:"name=APP_NAME,default=default-app"`
		Features []string      `envx:"name=FEATURES,default=logs,metrics"`
		Timeout  time.Duration `envx:"name=TIMEOUT,default=30s"`
		DBURL    string        `envx:"name=DB_URL,required=true,secret=true"`
		Mode     string        `envx:"name=MODE,default=local,enum=\"local,staging,production\""`
	}

	cfg, err := envx.Load[Config](
		envx.WithValidator(func(cfg *Config) error {
			if cfg.Port < 1024 {
				return errors.New("port must be >= 1024")
			}
			if len(cfg.Features) == 0 {
				return errors.New("features must not be empty")
			}
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Port != 7000 {
		t.Fatalf("Port = %d, want 7000", cfg.Port)
	}
	if cfg.AppName != "dotenv-app" {
		t.Fatalf("AppName = %q, want dotenv-app", cfg.AppName)
	}
	if cfg.Mode != "production" {
		t.Fatalf("Mode = %q, want production", cfg.Mode)
	}
	if cfg.DBURL != "postgres://dotenv/service" {
		t.Fatalf("DBURL = %q, want postgres://dotenv/service", cfg.DBURL)
	}
	if cfg.Timeout != 45*time.Second {
		t.Fatalf("Timeout = %v, want 45s", cfg.Timeout)
	}
	if len(cfg.Features) != 2 || cfg.Features[0] != "metrics" || cfg.Features[1] != "tracing" {
		t.Fatalf("unexpected Features: %#v", cfg.Features)
	}
}

func TestFunctionalLoad_PrecedenceModes(t *testing.T) {
	chdirTemp(t)

	if err := os.WriteFile(".env", []byte("PORT=5000\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("PORT", "6000")

	type Config struct {
		Port int `envx:"name=PORT,default=7000"`
	}

	cfgEnvWins, err := envx.Load[Config](
		envx.WithProvider(envx.Map(map[string]string{"PORT": "4000"})),
	)
	if err != nil {
		t.Fatalf("Load env wins: %v", err)
	}
	if cfgEnvWins.Port != 6000 {
		t.Fatalf("default precedence: Port = %d, want 6000", cfgEnvWins.Port)
	}

	cfgCustomWins, err := envx.Load[Config](
		envx.WithPrecedence(envx.PrecedenceCustomWins),
		envx.WithProvider(envx.Map(map[string]string{"PORT": "4000"})),
	)
	if err != nil {
		t.Fatalf("Load custom wins: %v", err)
	}
	if cfgCustomWins.Port != 4000 {
		t.Fatalf("custom precedence: Port = %d, want 4000", cfgCustomWins.Port)
	}
}

func TestFunctionalLoader_HotReload(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	if err := os.WriteFile(configPath, []byte(`{"port":3000,"name":"api"}`), 0644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	type Config struct {
		Port int    `envx:"name=PORT,default=8080"`
		Name string `envx:"name=NAME,default=svc"`
	}

	type reloadEvent struct {
		Old Config
		New Config
	}
	reloads := make(chan reloadEvent, 1)

	loader := envx.NewLoader[Config](
		envx.WithProvider(envx.File(configPath)),
		envx.WithWatch(configPath, 20*time.Millisecond),
		envx.WithOnReload(func(old *Config, new *Config) {
			if old == nil || new == nil {
				return
			}
			select {
			case reloads <- reloadEvent{Old: *old, New: *new}:
			default:
			}
		}),
	)

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("initial Load: %v", err)
	}
	if cfg.Port != 3000 || cfg.Name != "api" {
		t.Fatalf("unexpected initial cfg: %#v", cfg)
	}

	if err := loader.StartWatching(); err != nil {
		t.Fatalf("StartWatching: %v", err)
	}
	defer loader.StopWatching()

	time.Sleep(40 * time.Millisecond)
	if err := os.WriteFile(configPath, []byte(`{"port":3333,"name":"worker"}`), 0644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	select {
	case ev := <-reloads:
		if ev.Old.Port != 3000 || ev.New.Port != 3333 {
			t.Fatalf("unexpected reload event: old=%#v new=%#v", ev.Old, ev.New)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting reload event")
	}

	current := loader.Get()
	if current == nil {
		t.Fatal("loader.Get() returned nil after reload")
	}
	if current.Port != 3333 || current.Name != "worker" {
		t.Fatalf("unexpected current cfg: %#v", current)
	}
	if loader.Version() < 2 {
		t.Fatalf("expected version >= 2 after reload, got %d", loader.Version())
	}
}

func ExampleLoad_withPrecedence() {
	type Config struct {
		Port int `envx:"name=PORT,default=8080"`
	}

	_, _ = envx.Load[Config](
		envx.WithPrecedence(envx.PrecedenceCustomWins),
		envx.WithProvider(envx.Map(map[string]string{"PORT": "9000"})),
	)

	fmt.Println("ok")
	// Output: ok
}
