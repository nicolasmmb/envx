package envx

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func parse(cfg any, values map[string]any, prefix string) error {
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &Error{Field: "config", Err: fmt.Errorf("%w: target must be a non-nil pointer to a struct", ErrUnsupportedType)}
	}

	v := rv.Elem()
	if v.Kind() != reflect.Struct {
		return &Error{Field: "config", Err: fmt.Errorf("%w: target must point to a struct, got %s", ErrUnsupportedType, v.Kind())}
	}

	return parseStruct(v, v.Type(), "", values, prefix)
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
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}

func setField(fv reflect.Value, val any) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(fmt.Sprintf("%v", val))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			return setDuration(fv, val)
		}
		return setIntValue(fv, val)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return setUintValue(fv, val)

	case reflect.Float32, reflect.Float64:
		return setFloatValue(fv, val)

	case reflect.Bool:
		return setBoolValue(fv, val)

	case reflect.Slice:
		items, err := normalizeSliceInput(val)
		if err != nil {
			return err
		}
		slice := reflect.MakeSlice(fv.Type(), len(items), len(items))
		for i, item := range items {
			if err := setField(slice.Index(i), item); err != nil {
				return err
			}
		}
		fv.Set(slice)
		return nil

	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedType, fv.Kind())
	}
	return nil
}

func setDuration(fv reflect.Value, val any) error {
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
	return nil
}

func setIntValue(fv reflect.Value, val any) error {
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
	return nil
}

func setUintValue(fv reflect.Value, val any) error {
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
	return nil
}

func setFloatValue(fv reflect.Value, val any) error {
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
	return nil
}

func setBoolValue(fv reflect.Value, val any) error {
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
	return nil
}

func normalizeSliceInput(val any) ([]any, error) {
	if items, ok := val.([]any); ok {
		return items, nil
	}

	str, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("unsupported slice source type: %T", val)
	}

	parts := splitCSV(str)
	items := make([]any, len(parts))
	for i, p := range parts {
		items[i] = strings.TrimSpace(p)
	}
	return items, nil
}

func splitCSV(s string) []string {
	r := csv.NewReader(strings.NewReader(s))
	parts, err := r.Read()
	if err != nil {
		return strings.Split(s, ",")
	}
	return parts
}
