package envx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type envProvider struct{}

func Env() Provider {
	return &envProvider{}
}

func (envProvider) PrefixAware() bool { return true }

func (p *envProvider) Values() (map[string]any, error) {
	values := make(map[string]any)
	for _, env := range os.Environ() {
		if i := strings.Index(env, "="); i >= 0 {
			values[env[:i]] = env[i+1:]
		}
	}
	return values, nil
}

type defaultsProvider[T any] struct {
	prefix string
}

func (p *defaultsProvider[T]) PrefixAware() bool { return true }

func Defaults[T any]() Provider {
	return DefaultsWithPrefix[T]("")
}

func DefaultsWithPrefix[T any](prefix string) Provider {
	return &defaultsProvider[T]{prefix: strings.ToUpper(prefix)}
}

func (p *defaultsProvider[T]) Values() (map[string]any, error) {
	t, err := resolveStructType[T]()
	if err != nil {
		return nil, err
	}

	strDefaults := extractDefaults(t, "")

	values := make(map[string]any)
	for k, v := range strDefaults {
		key := k
		if p.prefix != "" {
			key = p.prefix + "_" + k
		}
		values[key] = v
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

type fileProvider struct {
	path string
}

func File(path string) Provider {
	absPath, _ := filepath.Abs(path)
	return &fileProvider{path: absPath}
}

func (p *fileProvider) Values() (map[string]any, error) {
	data, err := os.ReadFile(p.path)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(p.path))
	if ext == ".env" {
		strMap := parseDotEnv(data)
		values := make(map[string]any)
		for k, v := range strMap {
			values[k] = v
		}
		return values, nil
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	values := make(map[string]any)
	flattenMap("", raw, values)
	return values, nil
}

func parseDotEnv(data []byte) map[string]string {
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

		if len(val) >= 2 && ((strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'"))) {
			val = val[1 : len(val)-1]
		}

		values[key] = val
	}
	return values
}

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

			out[key] = val
		default:
			out[key] = val
		}
	}
}

type mapProvider struct {
	values map[string]string
}

func Map(values map[string]string) Provider {
	return &mapProvider{values: values}
}

func (mapProvider) PrefixAware() bool { return false }

func (p *mapProvider) Values() (map[string]any, error) {
	values := make(map[string]any)
	for k, v := range p.values {
		values[k] = v
	}
	return values, nil
}

// ============================================================================

func resolveStructType[T any]() (reflect.Type, error) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: configuration type must be a struct", ErrUnsupportedType)
	}

	return t, nil
}

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
