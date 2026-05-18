package envx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicolasmmb/envx"
)

// Regression tests for previously observed issues.

func TestIssueFixed_RequiredBoolFalseIsAccepted(t *testing.T) {
	type Config struct {
		Enabled bool `envx:"name=ENABLED,required=true"`
	}

	t.Setenv("ENABLED", "false")
	cfg, err := envx.Load[Config]()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.Enabled {
		t.Fatal("expected Enabled=false")
	}
}

func TestIssueFixed_RequiredIntZeroIsAccepted(t *testing.T) {
	type Config struct {
		Count int `envx:"name=COUNT,required=true"`
	}

	t.Setenv("COUNT", "0")
	cfg, err := envx.Load[Config]()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.Count != 0 {
		t.Fatalf("expected Count=0, got %d", cfg.Count)
	}
}

func TestIssueFixed_PrintNilDoesNotPanic(t *testing.T) {
	type Config struct {
		Port int
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected no panic when printing nil config, got %v", r)
		}
	}()

	var cfg *Config
	envx.Print(cfg)
}

func TestIssueFixed_FloatJSONForIntReturnsParseError(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080.9}`), 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	type Config struct {
		Port int `envx:"name=PORT"`
	}

	_, err := envx.Load[Config](envx.WithProvider(envx.File(tmpfile)))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !errors.Is(err, envx.ErrParse) {
		t.Fatalf("expected ErrParse, got %v", err)
	}
}

func TestIssueFixed_NegativeJSONForUintReturnsParseError(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"limit": -1}`), 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	type Config struct {
		Limit uint `envx:"name=LIMIT"`
	}

	_, err := envx.Load[Config](envx.WithProvider(envx.File(tmpfile)))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !errors.Is(err, envx.ErrParse) {
		t.Fatalf("expected ErrParse, got %v", err)
	}
}

func TestIssueFixed_NestedNamePatternFromDocsWorks(t *testing.T) {
	type Config struct {
		Database struct {
			URL string `envx:"name=DATABASE_URL,required=true"`
		}
	}

	t.Setenv("DATABASE_URL", "postgres://localhost/db")
	cfg, err := envx.Load[Config]()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg.Database.URL != "postgres://localhost/db" {
		t.Fatalf("unexpected URL: %q", cfg.Database.URL)
	}
}

func TestIssueFixed_RequiredErrorWithPrefixIncludesPrefix(t *testing.T) {
	type Config struct {
		Port int `envx:"name=PORT,required=true"`
	}

	_, err := envx.Load[Config](envx.WithPrefix("APP"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, envx.ErrRequired) {
		t.Fatalf("expected ErrRequired, got %v", err)
	}
	if !strings.Contains(err.Error(), "APP_PORT") {
		t.Fatalf("expected APP_PORT in error, got %v", err)
	}
}
