package envx

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type nativeEnv string

const (
	nativeEnvDev  nativeEnv = "dev"
	nativeEnvProd nativeEnv = "prod"
)

func (nativeEnv) EnumValues() []string {
	return []string{string(nativeEnvDev), string(nativeEnvProd)}
}

type nativeLevel int

const (
	nativeLevelDebug nativeLevel = iota
	nativeLevelInfo
	nativeLevelError
)

func (l nativeLevel) IsValid() bool {
	switch l {
	case nativeLevelDebug, nativeLevelInfo, nativeLevelError:
		return true
	default:
		return false
	}
}

type nativeTextLevel int

const (
	nativeTextLevelLow nativeTextLevel = iota + 1
	nativeTextLevelHigh
)

func (l nativeTextLevel) String() string {
	switch l {
	case nativeTextLevelLow:
		return "low"
	case nativeTextLevelHigh:
		return "high"
	default:
		return fmt.Sprintf("nativeTextLevel(%d)", int(l))
	}
}

func (l *nativeTextLevel) UnmarshalText(text []byte) error {
	switch string(text) {
	case "low":
		*l = nativeTextLevelLow
	case "high":
		*l = nativeTextLevelHigh
	default:
		return fmt.Errorf("invalid level %q", text)
	}
	return nil
}

type nativeStructEnum struct {
	Value string
}

func (e *nativeStructEnum) UnmarshalText(text []byte) error {
	switch string(text) {
	case "first", "second":
		e.Value = string(text)
		return nil
	default:
		return fmt.Errorf("invalid struct enum %q", text)
	}
}

func TestNativeEnum_WithoutTags(t *testing.T) {
	t.Setenv("NATIVE_ENUM_SAMPLE", "prod")

	type Config struct {
		NativeEnumSample nativeEnv
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.NativeEnumSample != nativeEnvProd {
		t.Fatalf("NativeEnumSample = %q, want %q", cfg.NativeEnumSample, nativeEnvProd)
	}
}

func TestNativeEnum_EnumValuesValid(t *testing.T) {
	t.Setenv("APP_ENV", "prod")

	type Config struct {
		Env nativeEnv `envx:"name=APP_ENV"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != nativeEnvProd {
		t.Fatalf("Env = %q, want %q", cfg.Env, nativeEnvProd)
	}
}

func TestNativeEnum_EnumValuesInvalid(t *testing.T) {
	t.Setenv("APP_ENV", "broken")

	type Config struct {
		Env nativeEnv `envx:"name=APP_ENV"`
	}

	_, err := Load[Config]()
	if err == nil {
		t.Fatal("expected enum validation error")
	}
	if !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse, got %v", err)
	}
	if !strings.Contains(err.Error(), "not in enum") {
		t.Fatalf("expected enum error message, got %v", err)
	}
	if !strings.Contains(err.Error(), "APP_ENV") {
		t.Fatalf("expected field name in error, got %v", err)
	}
}

func TestNativeEnum_EnumValidator(t *testing.T) {
	t.Setenv("LEVEL", "2")

	type Config struct {
		Level nativeLevel `envx:"name=LEVEL"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Level != nativeLevelError {
		t.Fatalf("Level = %d, want %d", cfg.Level, nativeLevelError)
	}

	t.Setenv("LEVEL", "9")
	if _, err := Load[Config](); !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse for invalid enum, got %v", err)
	}
}

func TestNativeEnum_TextUnmarshaler(t *testing.T) {
	t.Setenv("LEVEL", "high")

	type Config struct {
		Level nativeTextLevel `envx:"name=LEVEL"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Level != nativeTextLevelHigh {
		t.Fatalf("Level = %v, want high", cfg.Level)
	}

	t.Setenv("LEVEL", "nope")
	if _, err := Load[Config](); !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse for invalid enum, got %v", err)
	}
}

func TestNativeEnum_TextUnmarshalerStruct(t *testing.T) {
	t.Setenv("MODE", "second")

	type Config struct {
		Mode nativeStructEnum `envx:"name=MODE"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode.Value != "second" {
		t.Fatalf("Mode = %q, want second", cfg.Mode.Value)
	}
}

func TestNativeEnum_Pointer(t *testing.T) {
	t.Setenv("APP_ENV", "dev")

	type Config struct {
		Env *nativeEnv `envx:"name=APP_ENV"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env == nil || *cfg.Env != nativeEnvDev {
		t.Fatalf("Env = %v, want %q", cfg.Env, nativeEnvDev)
	}

	t.Setenv("APP_ENV", "broken")
	if _, err := Load[Config](); !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse for invalid enum, got %v", err)
	}
}

func TestNativeEnum_Slice(t *testing.T) {
	t.Setenv("ENVS", "dev,prod")

	type Config struct {
		Envs []nativeEnv `envx:"name=ENVS"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Envs) != 2 || cfg.Envs[0] != nativeEnvDev || cfg.Envs[1] != nativeEnvProd {
		t.Fatalf("Envs = %v", cfg.Envs)
	}

	t.Setenv("ENVS", "dev,broken")
	_, err = Load[Config]()
	if !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse for invalid enum, got %v", err)
	}
	if !strings.Contains(err.Error(), "ENVS[1]") {
		t.Fatalf("expected indexed field in error, got %v", err)
	}
}

func TestNativeEnum_NoValueSkipsValidation(t *testing.T) {
	type Config struct {
		Env nativeEnv `envx:"name=ENVX_NATIVE_ENUM_UNSET"`
	}

	cfg, err := Load[Config]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "" {
		t.Fatalf("Env = %q, want empty", cfg.Env)
	}
}

func TestNativeEnum_DefaultIsValidated(t *testing.T) {
	type Config struct {
		Env nativeEnv `envx:"name=ENVX_NATIVE_ENUM_DEFAULT,default=staging"`
	}

	if _, err := Load[Config](); !errors.Is(err, ErrParse) {
		t.Fatalf("expected ErrParse for invalid default, got %v", err)
	}
}

func TestNativeEnum_EnumValuesTakesPrecedence(t *testing.T) {
	t.Setenv("LEVEL", "1")

	type Config struct {
		Level nativeLevel `envx:"name=LEVEL"`
	}

	// nativeLevel only implements IsValid; this confirms the EnumValues path
	// is skipped cleanly when the type does not implement it.
	if _, err := Load[Config](); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
