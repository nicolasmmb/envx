package envx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// parse populates cfg from values map using prefix.
func parse(cfg any, values map[string]string, prefix string) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	return parseStruct(v, t, "", values, prefix)
}

func parseStruct(v reflect.Value, t reflect.Type, path string, values map[string]string, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		// Nested struct - convert field name to SCREAMING_SNAKE for the path
		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedPath := path + toScreamingSnake(field.Name) + "_"
			if err := parseStruct(fv, field.Type, nestedPath, values, prefix); err != nil {
				return err
			}
			continue
		}

		// Build key: PATH_FIELDNAME (already in SCREAMING_SNAKE)
		key := path + toScreamingSnake(field.Name)
		if prefix != "" {
			key = prefix + "_" + key
		}

		// Try with prefix, then without
		val, ok := values[key]
		if !ok && prefix != "" {
			val, ok = values[path+toScreamingSnake(field.Name)]
		}
		if !ok || val == "" {
			continue
		}

		if err := setField(fv, val); err != nil {
			return &Error{Field: key, Err: fmt.Errorf("%w: %v", ErrParse, err)}
		}
	}
	return nil
}

func validateRequired(cfg any) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	return checkRequired(v, t, "")
}

func checkRequired(v reflect.Value, t reflect.Type, path string) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedPath := path + toScreamingSnake(field.Name) + "_"
			if err := checkRequired(fv, field.Type, nestedPath); err != nil {
				return err
			}
			continue
		}

		if field.Tag.Get("required") == "true" && isZero(fv) {
			return &Error{Field: path + toScreamingSnake(field.Name), Err: ErrRequired}
		}
	}
	return nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func setField(fv reflect.Value, val string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return err
			}
			fv.SetInt(int64(d))
		} else {
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return err
			}
			fv.SetInt(n)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fv.SetUint(n)

	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		fv.SetFloat(n)

	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		fv.SetBool(b)

	case reflect.Slice:
		parts := strings.Split(val, ",")
		slice := reflect.MakeSlice(fv.Type(), len(parts), len(parts))
		for i, p := range parts {
			if err := setField(slice.Index(i), strings.TrimSpace(p)); err != nil {
				return err
			}
		}
		fv.Set(slice)

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, fv.Kind())
	}
	return nil
}

// toScreamingSnake converts CamelCase to SCREAMING_SNAKE_CASE.
// Handles acronyms: DatabaseURL -> DATABASE_URL, HTTPServer -> HTTP_SERVER
func toScreamingSnake(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := runes[i-1]
			// Add underscore if previous char was lowercase
			// OR if next char exists and is lowercase (end of acronym)
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
