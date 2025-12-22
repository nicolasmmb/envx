package envx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

func (p *envProvider) Values() (map[string]string, error) {
	values := make(map[string]string)
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

func (p *defaultsProvider[T]) Values() (map[string]string, error) {
	var cfg T
	return extractDefaults(reflect.TypeOf(cfg), ""), nil
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

// File returns a provider that reads from a JSON file.
func File(path string) Provider {
	absPath, _ := filepath.Abs(path)
	return &fileProvider{path: absPath}
}

func (p *fileProvider) Values() (map[string]string, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	values := make(map[string]string)
	flattenMap("", raw, values)
	return values, nil
}

func flattenMap(prefix string, m map[string]any, out map[string]string) {
	for k, v := range m {
		key := toScreamingSnake(k)
		if prefix != "" {
			key = prefix + "_" + key
		}

		switch val := v.(type) {
		case map[string]any:
			flattenMap(key, val, out)
		case []any:
			parts := make([]string, len(val))
			for i, item := range val {
				parts[i] = fmt.Sprintf("%v", item)
			}
			out[key] = strings.Join(parts, ",")
		case string:
			out[key] = val
		case float64:
			if val == float64(int64(val)) {
				out[key] = strconv.FormatInt(int64(val), 10)
			} else {
				out[key] = strconv.FormatFloat(val, 'f', -1, 64)
			}
		case bool:
			out[key] = strconv.FormatBool(val)
		case nil:
			// skip
		default:
			out[key] = fmt.Sprintf("%v", val)
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

func (p *mapProvider) Values() (map[string]string, error) {
	return p.values, nil
}
