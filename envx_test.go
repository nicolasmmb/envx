package envx

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type failingProvider struct{}

func (f failingProvider) Values() (map[string]any, error) {
	return nil, errors.New("boom")
}

type typeValidatedConfig struct {
	Port int `default:"8080"`
}

func (t typeValidatedConfig) Validate() error {
	return errors.New("invalid")
}

type lockedBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}

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

func TestLoad_UnsupportedType(t *testing.T) {
	_, err := Load[int]()
	if err == nil {
		t.Fatal("expected error for non-struct config")
	}

	if !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("expected ErrUnsupportedType, got %v", err)
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

func TestLoad_RequiredTime(t *testing.T) {
	type Config struct {
		StartedAt time.Time `required:"true"`
	}

	_, err := Load[Config]()
	if err == nil {
		t.Fatal("expected error for missing required time")
	}

	if !errors.Is(err, ErrRequired) {
		t.Fatalf("expected ErrRequired, got %v", err)
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

func TestLoad_SliceCSV(t *testing.T) {
	os.Setenv("ORIGINS", `"http://a.com","http://b.com,c.com"`)
	t.Cleanup(func() { os.Unsetenv("ORIGINS") })

	type Config struct {
		Origins []string
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Origins) != 2 {
		t.Fatalf("len(Origins) = %d, want 2", len(cfg.Origins))
	}
	if cfg.Origins[0] != "http://a.com" {
		t.Errorf("Origins[0] = %s, want http://a.com", cfg.Origins[0])
	}
	if cfg.Origins[1] != "http://b.com,c.com" {
		t.Errorf("Origins[1] = %s, want http://b.com,c.com", cfg.Origins[1])
	}
}

func TestLoad_DotEnv(t *testing.T) {
	content := `
# Comment
PORT=9090
HOST="127.0.0.1"
DEBUG=true
`
	tmpfile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(tmpfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port  int
		Host  string
		Debug bool
	}

	// Use File provider explicitly pointing to .env
	cfg, err := Load[Config](WithProvider(File(tmpfile)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %s, want 127.0.0.1", cfg.Host)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
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

func TestLoad_WithPrefix_IgnoresUnprefixed(t *testing.T) {
	os.Setenv("PORT", "9000")
	t.Cleanup(func() { os.Unsetenv("PORT") })

	type Config struct {
		Port int `default:"8080"`
	}

	cfg, err := Load[Config](WithPrefix("APP"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080 when unprefixed env is set", cfg.Port)
	}
}

func TestLoad_WithPrefixDefaults(t *testing.T) {
	type Config struct {
		Port int `default:"8080"`
	}

	cfg, err := Load[Config](WithPrefix("APP"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", cfg.Port)
	}
}

type togglingProvider struct {
	fail atomic.Bool
}

func (p *togglingProvider) Values() (map[string]any, error) {
	if p.fail.Load() {
		return nil, errors.New("provider failure")
	}
	return map[string]any{}, nil
}

func TestLoader_ReloadErrorIsLogged(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080}`), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port int `default:"8080"`
	}

	var buf lockedBuffer
	tp := &togglingProvider{}

	loader := NewLoader[Config](
		WithWatch(tmpfile, 10*time.Millisecond),
		WithProvider(File(tmpfile)),
		WithProvider(Defaults[Config]()),
		WithProvider(tp),
		WithOutput(&buf),
		WithOnReloadError(func(err error) {
			buf.Write([]byte("ERR:"))
			buf.Write([]byte(err.Error()))
		}),
	)

	loader.MustLoad()
	if err := loader.StartWatching(); err != nil {
		t.Fatalf("start watching: %v", err)
	}
	defer loader.StopWatching()

	// Next reload will fail
	tp.fail.Store(true)

	// Trigger reload
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(tmpfile, []byte(`{"port": 9090}`), 0644); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(500 * time.Millisecond)
	for {
		if bytes.Contains(buf.Bytes(), []byte("reload failed")) || bytes.Contains(buf.Bytes(), []byte("ERR:")) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("expected reload failure to be logged")
		default:
			time.Sleep(10 * time.Millisecond)
		}
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
		WithValidator(func(cfg *Config) error {
			if cfg.Port < 1024 {
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
		{"HTTPServer", "HTTP_SERVER"},
	}

	for _, tc := range tests {
		got := toScreamingSnake(tc.input)
		if got != tc.want {
			t.Errorf("toScreamingSnake(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLoader_Concurrency(t *testing.T) {
	loader := NewLoader[struct{}](WithWatch("config.json", 100*time.Millisecond))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_ = loader.StartWatching()
	}()

	go func() {
		defer wg.Done()
		_ = loader.StartWatching()
	}()

	wg.Wait()
	loader.StopWatching()
	loader.StopWatching()
}

func TestLoader_OnReload(t *testing.T) {
	// Create temp file
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	initialContent := `{"port": 8080, "debug": false}`
	if err := os.WriteFile(tmpfile, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port  int  `default:"8080"`
		Debug bool `default:"false"`
	}

	var mu sync.Mutex
	var oldCfg, newCfg *Config
	changesChan := make(chan struct{}, 1)

	// Callback
	onReload := func(old *Config, new *Config) {
		mu.Lock()
		oldCfg = old
		newCfg = new
		mu.Unlock()
		select {
		case changesChan <- struct{}{}:
		default:
		}
	}

	loader := NewLoader[Config](
		WithWatch(tmpfile, 50*time.Millisecond),
		WithProvider(File(tmpfile)),
		WithOnReload(onReload),
	)

	// Initial load
	loader.MustLoad()
	if err := loader.StartWatching(); err != nil {
		t.Fatalf("start watching: %v", err)
	}
	defer loader.StopWatching()

	// Modify file - Change Port
	newContent := `{"port": 9090, "debug": false}`
	time.Sleep(100 * time.Millisecond) // Ensure mtime passes
	if err := os.WriteFile(tmpfile, []byte(newContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Wait for reload
	select {
	case <-changesChan:
		mu.Lock()
		defer mu.Unlock()

		if oldCfg.Port != 8080 {
			t.Errorf("expected old Port 8080, got %d", oldCfg.Port)
		}
		if newCfg.Port != 9090 {
			t.Errorf("expected new Port 9090, got %d", newCfg.Port)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reload callback")
	}
}

func TestLoader_StartWatchingInvalidInterval(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080}`), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port int
	}

	loader := NewLoader[Config](WithWatch(tmpfile, 0), WithProvider(File(tmpfile)))
	loader.MustLoad()

	if err := loader.StartWatching(); err == nil {
		t.Fatal("expected error for non-positive watch interval")
	}

	if loader.Get().Port != 8080 {
		t.Fatalf("expected loaded config to remain, got %v", loader.Get())
	}
}

func TestLoader_StartWatchingFailsInitialLoad(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080}`), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port int
	}

	loader := NewLoader[Config](
		WithWatch(tmpfile, 50*time.Millisecond),
		WithProvider(failingProvider{}),
	)

	if err := loader.StartWatching(); err == nil {
		t.Fatal("expected error for failed initial load")
	}

	if loader.Get() != nil {
		t.Fatalf("expected config to stay nil after failed load, got %#v", loader.Get())
	}
}

type testLogger struct {
	msgs []string
}

func (l *testLogger) Printf(format string, args ...any) {
	l.msgs = append(l.msgs, fmt.Sprintf(format, args...))
}

func TestLoadFromEnv_UsesDotEnvAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if chErr := os.Chdir(oldwd); chErr != nil {
			t.Fatalf("restore cwd: %v", chErr)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile(".env", []byte("PORT=5000\nHOST=dotenv\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("PORT", "6000")

	type Config struct {
		Port int    `default:"7000"`
		Host string `default:"default"`
	}

	cfg, err := LoadFromEnv[Config]()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.Port != 6000 {
		t.Fatalf("expected env override port 6000, got %d", cfg.Port)
	}
	if cfg.Host != "dotenv" {
		t.Fatalf("expected dotenv host, got %q", cfg.Host)
	}
}

func TestMustLoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() {
		if chErr := os.Chdir(oldwd); chErr != nil {
			t.Fatalf("restore cwd: %v", chErr)
		}
	}()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile(".env", []byte("PORT=5050\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	type Config struct {
		Port int `default:"7000"`
	}

	cfg := MustLoadFromEnv[Config]()
	if cfg.Port != 5050 {
		t.Fatalf("expected port from dotenv, got %d", cfg.Port)
	}
}

func TestLoaderVersion(t *testing.T) {
	type Config struct {
		Port int `default:"8080"`
	}

	loader := NewLoader[Config]()
	if loader.Version() != 0 {
		t.Fatalf("expected version 0 before load, got %d", loader.Version())
	}

	loader.MustLoad()
	if loader.Version() != 1 {
		t.Fatalf("expected version 1 after load, got %d", loader.Version())
	}
}

func TestApplyPrefixForMapProvider(t *testing.T) {
	type Config struct {
		Port int
	}

	cfg, err := Load[Config](
		WithPrefix("APP"),
		WithProvider(Map(map[string]string{"PORT": "8081"})),
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8081 {
		t.Fatalf("expected prefixed port 8081, got %d", cfg.Port)
	}
}

func TestWithLogger(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080}`), 0644); err != nil {
		t.Fatal(err)
	}

	type Config struct {
		Port int
	}

	logger := &testLogger{}
	loader := NewLoader[Config](
		WithLogger(logger),
		WithProvider(File(tmpfile)),
		WithWatch(tmpfile, 0),
	)
	loader.MustLoad()

	if err := loader.StartWatching(); err == nil {
		t.Fatal("expected error for non-positive watch interval")
	}
	if len(logger.msgs) == 0 {
		t.Fatal("expected logger to be called")
	}
}

func TestPrintUsesStdout(t *testing.T) {
	type Config struct {
		Port int `default:"8080"`
	}
	cfg := &Config{Port: 8080}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	Print(cfg)
	_ = w.Close()
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Contains(out, []byte("PORT")) {
		t.Fatalf("expected output to include PORT, got %q", string(out))
	}
}

func TestParserCoversUintFloatDurationAndSlices(t *testing.T) {
	type Config struct {
		Rate  float64
		Limit uint
		Tags  []string
	}

	cfg, err := Load[Config](
		WithProvider(Map(map[string]string{
			"RATE":  "3.5",
			"LIMIT": "42",
			"TAGS":  "a,\"b,c\"",
		})),
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Rate != 3.5 {
		t.Fatalf("expected rate 3.5, got %v", cfg.Rate)
	}
	if cfg.Limit != 42 {
		t.Fatalf("expected limit 42, got %d", cfg.Limit)
	}
	if len(cfg.Tags) != 2 || cfg.Tags[1] != "b,c" {
		t.Fatalf("unexpected tags: %#v", cfg.Tags)
	}

	var d time.Duration
	fv := reflect.ValueOf(&d).Elem()
	if err := setDuration(fv, int64(10)); err != nil {
		t.Fatalf("setDuration int64: %v", err)
	}
	if err := setDuration(fv, float64(20)); err != nil {
		t.Fatalf("setDuration float64: %v", err)
	}

	var i int
	iv := reflect.ValueOf(&i).Elem()
	if err := setIntValue(iv, int32(7)); err != nil {
		t.Fatalf("setIntValue int32: %v", err)
	}

	var b bool
	bv := reflect.ValueOf(&b).Elem()
	if err := setBoolValue(bv, true); err != nil {
		t.Fatalf("setBoolValue bool: %v", err)
	}

	if err := setFloatValue(reflect.ValueOf(&cfg.Rate).Elem(), float64(9.5)); err != nil {
		t.Fatalf("setFloatValue float64: %v", err)
	}

	if _, err := normalizeSliceInput(123); err == nil {
		t.Fatal("expected normalizeSliceInput to fail for non-string slice source")
	}
}

func TestUtilityCoverage(t *testing.T) {
	if maskSecretValue("short") != "***" {
		t.Fatal("expected short secret to be masked")
	}
	if !strings.Contains(maskSecretValue("supersecretvalue"), "***") {
		t.Fatal("expected long secret to be masked")
	}

	errStr := (&Error{Field: "field", Err: ErrRequired}).Error()
	if !strings.Contains(errStr, "field") {
		t.Fatalf("unexpected error string: %s", errStr)
	}

	if isZero(reflect.ValueOf(1)) {
		t.Fatal("expected non-zero value to be false for isZero")
	}

	provider := Map(map[string]string{})
	if mp, ok := provider.(*mapProvider); ok {
		if mp.PrefixAware() {
			t.Fatal("expected mapProvider to be not prefix-aware")
		}
	} else {
		t.Fatal("expected mapProvider type")
	}

	out := make(map[string]any)
	flattenMap("", map[string]any{
		"app": map[string]any{
			"ports": []any{"1", "2"},
			"name":  "svc",
		},
	}, out)
	if _, ok := out["APP_PORTS"]; !ok {
		t.Fatalf("expected APP_PORTS in flattened map, got %#v", out)
	}

	parts := splitCSV(`a,"b`)
	if len(parts) != 2 || parts[1] != "\"b" {
		t.Fatalf("expected split fallback, got %#v", parts)
	}
}

func TestMoreCoverageBranches(t *testing.T) {
	var u uint
	uv := reflect.ValueOf(&u).Elem()
	if err := setUintValue(uv, float64(9)); err != nil {
		t.Fatalf("setUintValue float64: %v", err)
	}
	if err := setUintValue(uv, uint32(7)); err != nil {
		t.Fatalf("setUintValue uint32: %v", err)
	}
	if err := setUintValue(uv, "11"); err != nil {
		t.Fatalf("setUintValue string: %v", err)
	}
	if err := setUintValue(uv, "bad"); err == nil {
		t.Fatal("expected setUintValue string parse error")
	}
	if err := setUintValue(uv, true); err == nil {
		t.Fatal("expected setUintValue default error")
	}

	var f float64
	if err := setFloatValue(reflect.ValueOf(&f).Elem(), "2.5"); err != nil {
		t.Fatalf("setFloatValue string: %v", err)
	}
	if err := setFloatValue(reflect.ValueOf(&f).Elem(), "bad"); err == nil {
		t.Fatal("expected setFloatValue parse error")
	}
	if err := setFloatValue(reflect.ValueOf(&f).Elem(), 1); err == nil {
		t.Fatal("expected setFloatValue default error")
	}

	var d time.Duration
	if err := setDuration(reflect.ValueOf(&d).Elem(), "5s"); err != nil {
		t.Fatalf("setDuration string: %v", err)
	}
	if err := setDuration(reflect.ValueOf(&d).Elem(), "bad"); err == nil {
		t.Fatal("expected setDuration parse error")
	}
	if err := setDuration(reflect.ValueOf(&d).Elem(), 1); err == nil {
		t.Fatal("expected setDuration default error")
	}

	var i int
	if err := setIntValue(reflect.ValueOf(&i).Elem(), float64(3)); err != nil {
		t.Fatalf("setIntValue float64: %v", err)
	}
	if err := setIntValue(reflect.ValueOf(&i).Elem(), "bad"); err == nil {
		t.Fatal("expected setIntValue parse error")
	}
	if err := setIntValue(reflect.ValueOf(&i).Elem(), true); err == nil {
		t.Fatal("expected setIntValue default error")
	}

	var b bool
	if err := setBoolValue(reflect.ValueOf(&b).Elem(), "true"); err != nil {
		t.Fatalf("setBoolValue string: %v", err)
	}
	if err := setBoolValue(reflect.ValueOf(&b).Elem(), "notabool"); err == nil {
		t.Fatal("expected setBoolValue parse error")
	}
	if err := setBoolValue(reflect.ValueOf(&b).Elem(), 1); err == nil {
		t.Fatal("expected setBoolValue default error")
	}

	if _, err := normalizeSliceInput([]any{"a", "b"}); err != nil {
		t.Fatalf("normalizeSliceInput slice: %v", err)
	}

	if got := applyPrefix(map[string]any{"A": 1}, ""); got["A"] != 1 {
		t.Fatal("expected applyPrefix to return input map when prefix empty")
	}

	if isZero(reflect.Value{}) != true {
		t.Fatal("expected zero reflect.Value to be zero")
	}

	if err := wrapValidationError(nil); err != nil {
		t.Fatal("expected wrapValidationError nil to return nil")
	}

	logger := newWriterLogger(nil)
	logger.Printf("test")
	_ = newWriterLogger(&bytes.Buffer{})

	type Config struct {
		Port   int
		hidden string
	}

	values := map[string]any{"PORT": "8080", "HIDDEN": "ignored"}
	cfg := &Config{}
	if err := parse(cfg, values, ""); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Port != 8080 {
		t.Fatalf("expected port to be set, got %d", cfg.Port)
	}
	if cfg.hidden != "" {
		t.Fatalf("expected hidden field to remain empty, got %q", cfg.hidden)
	}

	if err := parse(123, values, ""); err == nil {
		t.Fatal("expected parse to fail on non-pointer target")
	}

	var nilCfg *Config
	if err := parse(nilCfg, values, ""); err == nil {
		t.Fatal("expected parse to fail on nil pointer")
	}

	var notStruct int
	if err := parse(&notStruct, values, ""); err == nil {
		t.Fatal("expected parse to fail on non-struct pointer")
	}

	if err := setField(reflect.ValueOf(&struct{ C complex64 }{}).Elem().Field(0), complex64(1)); err == nil {
		t.Fatal("expected setField to fail for unsupported kind")
	}

	var sliceHolder []string
	if err := setField(reflect.ValueOf(&sliceHolder).Elem(), []any{"a", "b"}); err != nil {
		t.Fatalf("setField slice []any: %v", err)
	}
	if err := setField(reflect.ValueOf(&sliceHolder).Elem(), 123); err == nil {
		t.Fatal("expected setField to fail for unsupported slice source type")
	}

	if _, err := resolveStructType[int](); err == nil {
		t.Fatal("expected resolveStructType to fail for non-struct type")
	}
	if _, err := resolveStructType[*Config](); err != nil {
		t.Fatalf("expected resolveStructType to succeed for pointer type: %v", err)
	}

	tmpfile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port":`), 0644); err != nil {
		t.Fatalf("write bad json: %v", err)
	}
	provider := File(tmpfile)
	if _, err := provider.Values(); err == nil {
		t.Fatal("expected file provider to fail on invalid json")
	}

	missing := File(filepath.Join(t.TempDir(), "missing.json"))
	if vals, err := missing.Values(); err != nil || vals != nil {
		t.Fatalf("expected missing file to return nil, got vals=%v err=%v", vals, err)
	}

	dotenv := File(filepath.Join(t.TempDir(), ".env"))
	if err := os.WriteFile(dotenv.(*fileProvider).path, []byte("KEY=\"value\""), 0644); err != nil {
		t.Fatalf("write dotenv: %v", err)
	}
	if vals, err := dotenv.Values(); err != nil || vals["KEY"] != "value" {
		t.Fatalf("expected dotenv value, got vals=%v err=%v", vals, err)
	}

	opt := WithValidator(func(cfg *Config) error { return nil })
	o := &options{}
	opt(o)
	if err := o.validator(cfg); err != nil {
		t.Fatalf("expected validator to succeed, got %v", err)
	}
	if err := o.validator(&struct{}{}); err == nil {
		t.Fatal("expected validator type mismatch error")
	}
}

func TestMustLoadFromEnvPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected MustLoadFromEnv to panic on invalid type")
		}
	}()
	_ = MustLoadFromEnv[int]()
}

func TestLoaderMustLoadPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Loader.MustLoad to panic on provider error")
		}
	}()

	type Config struct {
		Port int
	}

	loader := NewLoader[Config](WithProvider(failingProvider{}))
	_ = loader.MustLoad()
}

func TestPrintStructNested(t *testing.T) {
	type Nested struct {
		Name string `default:"svc"`
	}
	type Config struct {
		App  Nested
		Time time.Time
	}

	cfg := &Config{App: Nested{Name: "api"}, Time: time.Now()}
	var buf bytes.Buffer
	PrintTo(&buf, cfg)
	if !strings.Contains(buf.String(), "App:") {
		t.Fatalf("expected nested struct to be printed, got %q", buf.String())
	}
}

func TestParseStructPrefixAndRequired(t *testing.T) {
	type Config struct {
		Port int `required:"true"`
	}

	cfg := &Config{}
	values := map[string]any{"APP_PORT": "8088"}
	if err := parse(cfg, values, "APP"); err != nil {
		t.Fatalf("parse with prefix: %v", err)
	}
	if cfg.Port != 8088 {
		t.Fatalf("expected port 8088, got %d", cfg.Port)
	}

	cfg = &Config{}
	if err := validateRequired(cfg); err == nil {
		t.Fatal("expected required validation error")
	}
}

func TestParseStructNestedAndNilValue(t *testing.T) {
	type Nested struct {
		Name string
	}
	type Config struct {
		Port int
		Nest Nested
	}

	cfg := &Config{}
	values := map[string]any{
		"PORT":      nil,
		"NEST_NAME": "svc",
	}
	if err := parse(cfg, values, ""); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Nest.Name != "svc" {
		t.Fatalf("expected nested name to be set, got %q", cfg.Nest.Name)
	}
	if cfg.Port != 0 {
		t.Fatalf("expected port to remain zero, got %d", cfg.Port)
	}
}

func TestValidateRequiredNested(t *testing.T) {
	type Config struct {
		Nest struct {
			Token string `required:"true"`
		}
	}

	cfg := &Config{}
	if err := validateRequired(cfg); err == nil {
		t.Fatal("expected required error for nested field")
	}
	cfg.Nest.Token = "ok"
	if err := validateRequired(cfg); err != nil {
		t.Fatalf("expected no error for nested required, got %v", err)
	}
}

func TestReloadConfigBranches(t *testing.T) {
	type Config struct {
		Port int `default:"8080"`
	}

	loader := NewLoader[Config](WithProvider(Defaults[Config]()))
	loader.MustLoad()

	loader.opts = []Option{WithProvider(Defaults[Config]())}
	o := defaultOptions()
	finalizeOptions[Config](o)
	loader.reloadConfig(o)

	loader.opts = []Option{WithProvider(failingProvider{})}
	loader.reloadConfig(o)
}

func TestSetFieldSliceInvalidCSV(t *testing.T) {
	var sliceHolder []string
	if err := setField(reflect.ValueOf(&sliceHolder).Elem(), `a,"b`); err != nil {
		t.Fatalf("setField invalid csv fallback: %v", err)
	}
	if len(sliceHolder) != 2 {
		t.Fatalf("expected 2 items from fallback, got %#v", sliceHolder)
	}
}

func TestFileProviderReadError(t *testing.T) {
	dir := t.TempDir()
	provider := File(dir)
	if _, err := provider.Values(); err == nil {
		t.Fatal("expected error when reading directory as file")
	}
}

func TestStartWatchingNoPathAndTwice(t *testing.T) {
	type Config struct {
		Port int `default:"8080"`
	}

	loader := NewLoader[Config]()
	if err := loader.StartWatching(); err != nil {
		t.Fatalf("expected nil for empty watch path, got %v", err)
	}

	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080}`), 0644); err != nil {
		t.Fatal(err)
	}
	loader = NewLoader[Config](WithProvider(File(tmpfile)), WithWatch(tmpfile, 10*time.Millisecond))
	loader.MustLoad()
	if err := loader.StartWatching(); err != nil {
		t.Fatalf("start watching: %v", err)
	}
	defer loader.StopWatching()
	if err := loader.StartWatching(); err != nil {
		t.Fatalf("expected second StartWatching to be nil, got %v", err)
	}
}

func TestLoadInternalProviderError(t *testing.T) {
	type Config struct {
		Port int
	}

	if _, err := Load[Config](WithProvider(failingProvider{})); err == nil {
		t.Fatal("expected Load to return provider error")
	}
}

func TestLoadInternalErrors(t *testing.T) {
	type BadConfig struct {
		Value complex64
	}

	if _, err := Load[BadConfig](WithProvider(Map(map[string]string{"VALUE": "1"}))); err == nil {
		t.Fatal("expected parse error for unsupported type")
	}

	type Validated struct {
		Port int `default:"8080"`
	}

	if _, err := Load[Validated](
		WithProvider(Defaults[Validated]()),
		WithValidator(func(cfg *Validated) error { return errors.New("invalid") }),
	); err == nil {
		t.Fatal("expected option validator error")
	}

	if _, err := Load[typeValidatedConfig](WithProvider(Defaults[typeValidatedConfig]())); err == nil {
		t.Fatal("expected type validator error")
	}
}

func TestParseStructNonSettable(t *testing.T) {
	type Config struct {
		Port int
	}

	v := reflect.ValueOf(Config{})
	if err := parseStruct(v, v.Type(), "", map[string]any{"PORT": "8080"}, ""); err != nil {
		t.Fatalf("parseStruct non-settable: %v", err)
	}
}

func TestParseStructNestedError(t *testing.T) {
	type Nested struct {
		Bad complex64
	}
	type Config struct {
		Nest Nested
	}

	cfg := &Config{}
	values := map[string]any{"NEST_BAD": "1"}
	if err := parse(cfg, values, ""); err == nil {
		t.Fatal("expected parse to fail for nested unsupported type")
	}
}

func TestSetFieldDuration(t *testing.T) {
	var d time.Duration
	if err := setField(reflect.ValueOf(&d).Elem(), "2s"); err != nil {
		t.Fatalf("setField duration: %v", err)
	}
	if d != 2*time.Second {
		t.Fatalf("expected 2s duration, got %v", d)
	}
}

func TestSetFieldSliceItemError(t *testing.T) {
	var sliceHolder []int
	if err := setField(reflect.ValueOf(&sliceHolder).Elem(), []any{map[string]any{"x": 1}}); err == nil {
		t.Fatal("expected setField to fail for invalid slice item")
	}
}

func TestFileProviderValuesJSONSuccess(t *testing.T) {
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 8080, "nested": {"name": "api"}}`), 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}
	provider := File(tmpfile)
	values, err := provider.Values()
	if err != nil {
		t.Fatalf("Values: %v", err)
	}
	if values["PORT"] != float64(8080) || values["NESTED_NAME"] != "api" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestFinalizeOptionsLoggerOnly(t *testing.T) {
	type Config struct{}

	o := &options{providers: []Provider{Env()}}
	finalizeOptions[Config](o)
	if o.logger == nil {
		t.Fatal("expected logger to be set")
	}
}

func TestParseDotEnvBranches(t *testing.T) {
	data := []byte(`
# comment
NOEQ
KEY="value"
OTHER='x'
PLAIN=ok
`)
	values := parseDotEnv(data)
	if values["KEY"] != "value" || values["OTHER"] != "x" || values["PLAIN"] != "ok" {
		t.Fatalf("unexpected dotenv values: %#v", values)
	}
}

func TestWatchLoopBranches(t *testing.T) {
	type Config struct{}

	o := defaultOptions()
	o.watchEvery = time.Millisecond
	tmpfile := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmpfile, []byte(`{"port": 1}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	o.watchPath = tmpfile

	loader := &Loader[Config]{}
	stop := make(chan struct{})
	close(stop)
	var wg sync.WaitGroup
	wg.Add(1)
	newWatchLoop(loader, o, os.Stat).run(stop, &wg)
	wg.Wait()

	stop = make(chan struct{})
	wg.Add(1)
	go func() {
		time.Sleep(5 * time.Millisecond)
		close(stop)
	}()
	errStat := func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	newWatchLoop(loader, o, errStat).run(stop, &wg)
	wg.Wait()
}
