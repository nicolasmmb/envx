package envx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// ============================================================================
// Environment Provider
// ============================================================================

type envProvider struct{}

// Env returns a provider that reads from environment variables.
func Env() Provider {
	return &envProvider{}
}

func (p *envProvider) Values() (map[string]any, error) {
	values := make(map[string]any)
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i >= 0 {
			values[env[:i]] = env[i+1:]
		}
	}
	return values, nil
}

// ============================================================================
// Defaults Provider
// ============================================================================

type defaultsProvider[T any] struct{}

// Defaults returns a provider that reads default values from struct tags.
func Defaults[T any]() Provider {
	return &defaultsProvider[T]{}
}

func (p *defaultsProvider[T]) Values() (map[string]any, error) {
	var cfg T
	strDefaults := extractDefaults(reflect.TypeOf(cfg), "")

	values := make(map[string]any)
	for k, v := range strDefaults {
		values[k] = v
	}
	return values, nil
}

func extractDefaults(t reflect.Type, path string) map[string]string {
	values := make(map[string]string)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedPath := path + toScreamingSnake(field.Name) + "_"
			for k, v := range extractDefaults(field.Type, nestedPath) {
				values[k] = v
			}
			continue
		}

		if def := field.Tag.Get("default"); def != "" {
			values[path+toScreamingSnake(field.Name)] = def
		}
	}
	return values
}

// ============================================================================
// File Provider
// ============================================================================

type fileProvider struct {
	path string
}

// File returns a provider that reads from a file (JSON or .env).
func File(path string) Provider {
	absPath, _ := filepath.Abs(path)
	return &fileProvider{path: absPath}
}

func (p *fileProvider) Values() (map[string]any, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(p.path))
	if ext == ".env" {
		strMap, err := parseDotEnv(data)
		if err != nil {
			return nil, err
		}
		values := make(map[string]any)
		for k, v := range strMap {
			values[k] = v
		}
		return values, nil
	}

	// Default to JSON
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	values := make(map[string]any)
	flattenMap("", raw, values)
	return values, nil
}

func parseDotEnv(data []byte) (map[string]string, error) {
	values := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Remove quotes if present
		if len(val) >= 2 {
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
		}

		values[key] = val
	}
	return values, nil
}

// flattenMap now preserves types where possible, but keys are flattened.
// Since keys are strings, the map is map[string]any.
func flattenMap(prefix string, m map[string]any, out map[string]any) {
	for k, v := range m {
		key := toScreamingSnake(k)
		if prefix != "" {
			key = prefix + "_" + key
		}

		switch val := v.(type) {
		case map[string]any:
			flattenMap(key, val, out)
		case []any:
			// For slices, kept as is for now, parser will handle []any -> []type
			// Or we could flatten slices too? Current logic used to join strings.
			// Let's defer slice logic to 'parse' or keep it simple.
			// Ideally we shouldn't flatten slices into strings if we have any.
			out[key] = val
		default:
			out[key] = val
		}
	}
}

// ============================================================================
// Map Provider
// ============================================================================

type mapProvider struct {
	values map[string]string
}

// Map returns a provider from a string map.
func Map(values map[string]string) Provider {
	return &mapProvider{values: values}
}

func (p *mapProvider) Values() (map[string]any, error) {
	values := make(map[string]any)
	for k, v := range p.values {
		values[k] = v
	}
	return values, nil
}

// ============================================================================
// Helpers
// ============================================================================

// toScreamingSnake converts CamelCase to SCREAMING_SNAKE_CASE.
func toScreamingSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			if prev >= 'a' && prev <= 'z' {
				b.WriteByte('_')
			} else if i+1 < len(runes) {
				next := runes[i+1]
				if next >= 'a' && next <= 'z' {
					b.WriteByte('_')
				}
			}
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}
