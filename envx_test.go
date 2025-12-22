package envx

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	type Config struct {
		Port    int           `default:"8080"`
		Host    string        `default:"localhost"`
		Debug   bool          `default:"false"`
		Timeout time.Duration `default:"30s"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", cfg.Host)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestLoad_Env(t *testing.T) {
	os.Setenv("PORT", "3000")
	os.Setenv("DEBUG", "true")
	t.Cleanup(func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DEBUG")
	})

	type Config struct {
		Port  int  `default:"8080"`
		Debug bool `default:"false"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoad_Required(t *testing.T) {
	type Config struct {
		DatabaseURL string `required:"true"`
	}

	_, err := Load[Config]()
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
}

func TestLoad_RequiredWithValue(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/db")
	t.Cleanup(func() { os.Unsetenv("DATABASE_URL") })

	type Config struct {
		DatabaseURL string `required:"true"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/db" {
		t.Errorf("DatabaseURL = %s, want postgres://localhost/db", cfg.DatabaseURL)
	}
}

func TestLoad_NestedStruct(t *testing.T) {
	os.Setenv("DATABASE_HOST", "db.example.com")
	os.Setenv("DATABASE_PORT", "5432")
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_HOST")
		os.Unsetenv("DATABASE_PORT")
	})

	type Config struct {
		Database struct {
			Host string `default:"localhost"`
			Port int    `default:"3306"`
		}
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host = %s, want db.example.com", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
	}
}

func TestLoad_Slice(t *testing.T) {
	os.Setenv("HOSTS", "host1,host2,host3")
	t.Cleanup(func() { os.Unsetenv("HOSTS") })

	type Config struct {
		Hosts []string
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Hosts) != 3 {
		t.Fatalf("len(Hosts) = %d, want 3", len(cfg.Hosts))
	}
	if cfg.Hosts[0] != "host1" {
		t.Errorf("Hosts[0] = %s, want host1", cfg.Hosts[0])
	}
}

func TestLoad_WithPrefix(t *testing.T) {
	os.Setenv("APP_PORT", "9000")
	t.Cleanup(func() { os.Unsetenv("APP_PORT") })

	type Config struct {
		Port int `default:"8080"`
	}

	cfg, err := Load[Config](WithPrefix("APP"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
}

func TestLoad_WithProvider(t *testing.T) {
	type Config struct {
		Port int    `default:"8080"`
		Host string `default:"localhost"`
	}

	cfg, err := Load[Config](
		WithProvider(Defaults[Config]()),
		WithProvider(Map(map[string]string{
			"PORT": "5000",
			"HOST": "0.0.0.0",
		})),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 5000 {
		t.Errorf("Port = %d, want 5000", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %s, want 0.0.0.0", cfg.Host)
	}
}

func TestLoad_WithValidator(t *testing.T) {
	type Config struct {
		Port int `default:"80"`
	}

	_, err := Load[Config](
		WithValidator(func(cfg any) error {
			c := cfg.(*Config)
			if c.Port < 1024 {
				return ErrValidation
			}
			return nil
		}),
	)

	if err == nil {
		t.Fatal("expected validation error")
	}
}

type validatableConfig struct {
	Port int `default:"80"`
}

func (c *validatableConfig) Validate() error {
	if c.Port < 1024 {
		return ErrValidation
	}
	return nil
}

func TestLoad_ValidatorInterface(t *testing.T) {
	_, err := Load[validatableConfig]()
	if err == nil {
		t.Fatal("expected validation error from Validate() method")
	}
}

func TestLoad_Duration(t *testing.T) {
	os.Setenv("TIMEOUT", "5m30s")
	t.Cleanup(func() { os.Unsetenv("TIMEOUT") })

	type Config struct {
		Timeout time.Duration `default:"30s"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := 5*time.Minute + 30*time.Second
	if cfg.Timeout != want {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, want)
	}
}

func TestMustLoad_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()

	type Config struct {
		Required string `required:"true"`
	}

	MustLoad[Config]()
}

func TestPrint_MasksSecrets(t *testing.T) {
	type Config struct {
		Port      int    `default:"8080"`
		JWTSecret string `default:"supersecretkey123" secret:"true"`
		Password  string `default:"mypassword"`
	}

	cfg := MustLoad[Config]()

	var buf bytes.Buffer
	PrintTo(&buf, cfg)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("8080")) {
		t.Error("expected Port to be visible")
	}
	if bytes.Contains([]byte(output), []byte("supersecretkey123")) {
		t.Error("expected JWTSecret to be masked")
	}
	if bytes.Contains([]byte(output), []byte("mypassword")) {
		t.Error("expected Password to be masked")
	}
}

func TestToScreamingSnake(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Port", "PORT"},
		{"DatabaseURL", "DATABASE_URL"},
		{"JWTSecret", "JWT_SECRET"},
		{"HTTPServer", "HTTP_SERVER"},
	}

	for _, tc := range tests {
		got := toScreamingSnake(tc.input)
		if got != tc.want {
			t.Errorf("toScreamingSnake(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
