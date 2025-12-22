package envx

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// parse populates cfg from values map using prefix.
func parse(cfg any, values map[string]any, prefix string) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	return parseStruct(v, t, "", values, prefix)
}

func parseStruct(v reflect.Value, t reflect.Type, path string, values map[string]any, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedPath := path + toScreamingSnake(field.Name) + "_"
			if err := parseStruct(fv, field.Type, nestedPath, values, prefix); err != nil {
				return err
			}
			continue
		}

		key := path + toScreamingSnake(field.Name)
		if prefix != "" {
			key = prefix + "_" + key
		}

		val, ok := values[key]
		if !ok && prefix != "" {
			val, ok = values[path+toScreamingSnake(field.Name)]
		}
		if !ok || val == nil {
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

func setField(fv reflect.Value, val any) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(fmt.Sprintf("%v", val))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			switch v := val.(type) {
			case string:
				d, err := time.ParseDuration(v)
				if err != nil {
					return err
				}
				fv.SetInt(int64(d))
			case int64:
				fv.SetInt(v)
			case float64:
				fv.SetInt(int64(v))
			default:
				return fmt.Errorf("invalid duration type: %T", val)
			}
		} else {
			switch v := val.(type) {
			case float64:
				fv.SetInt(int64(v))
			case int, int8, int16, int32, int64:
				fv.SetInt(reflect.ValueOf(v).Int())
			case string:
				n, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return err
				}
				fv.SetInt(n)
			default:
				return fmt.Errorf("invalid int value: %v", val)
			}
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v := val.(type) {
		case float64:
			fv.SetUint(uint64(v))
		case uint, uint8, uint16, uint32, uint64:
			fv.SetUint(reflect.ValueOf(v).Uint())
		case string:
			n, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
			fv.SetUint(n)
		default:
			return fmt.Errorf("invalid uint value: %v", val)
		}

	case reflect.Float32, reflect.Float64:
		switch v := val.(type) {
		case float64:
			fv.SetFloat(v)
		case string:
			n, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return err
			}
			fv.SetFloat(n)
		default:
			return fmt.Errorf("invalid float value: %v", val)
		}

	case reflect.Bool:
		switch v := val.(type) {
		case bool:
			fv.SetBool(v)
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			fv.SetBool(b)
		default:
			return fmt.Errorf("invalid bool value: %v", val)
		}

	case reflect.Slice:
		// Handle []any (from JSON/Config) or string (from Env/CSV)
		var strParts []string

		switch v := val.(type) {
		case []any:
			// If we got a real slice from JSON
			slice := reflect.MakeSlice(fv.Type(), len(v), len(v))
			for i, item := range v {
				if err := setField(slice.Index(i), item); err != nil {
					return err
				}
			}
			fv.Set(slice)
			return nil
		case string:
			// Fallback to CSV parsing for strings
			r := csv.NewReader(strings.NewReader(v))
			parts, err := r.Read()
			if err != nil {
				parts = strings.Split(v, ",")
			}
			strParts = parts
		default:
			return fmt.Errorf("unsupported slice source type: %T", val)
		}

		slice := reflect.MakeSlice(fv.Type(), len(strParts), len(strParts))
		for i, p := range strParts {
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
