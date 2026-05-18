package envx

import (
	"encoding/csv"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func parse(cfg any, values map[string]any, prefix string, logger Logger) error {
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &Error{Field: "config", Err: fmt.Errorf("%w: target must be a non-nil pointer to a struct", ErrUnsupportedType)}
	}

	v := rv.Elem()
	if v.Kind() != reflect.Struct {
		return &Error{Field: "config", Err: fmt.Errorf("%w: target must point to a struct, got %s", ErrUnsupportedType, v.Kind())}
	}

	if logger == nil {
		logger = defaultOptions().logger
	}

	return parseStruct(v, v.Type(), "", values, prefix, logger)
}

func parseStruct(v reflect.Value, t reflect.Type, path string, values map[string]any, prefix string, logger Logger) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		tag := parseEnvxTag(field.Tag.Get("envx"))
		if tag.Skip {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedName := toScreamingSnake(field.Name)
			if tag.Name != "" {
				nestedName = tag.Name
			}
			nestedPath := path + nestedName + "_"
			if err := parseStruct(fv, field.Type, nestedPath, values, prefix, logger); err != nil {
				return err
			}
			continue
		}
		if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct && field.Type.Elem() != reflect.TypeOf(time.Time{}) {
			nestedName := toScreamingSnake(field.Name)
			if tag.Name != "" {
				nestedName = tag.Name
			}
			nestedPath := path + nestedName + "_"
			nestedKeyPrefix := withPrefix(prefix, nestedPath)

			if fv.IsNil() {
				if !hasAnyKeyWithPrefix(values, nestedKeyPrefix) {
					continue
				}
				fv.Set(reflect.New(field.Type.Elem()))
			}

			if err := parseStruct(fv.Elem(), field.Type.Elem(), nestedPath, values, prefix, logger); err != nil {
				return err
			}
			continue
		}

		key := withPrefix(prefix, fieldKey(path, field.Name, tag.Name))

		val, ok := values[key]
		if !ok || val == nil {
			continue
		}

		if tag.Deprecated {
			logDeprecatedKey(logger, key)
		}

		if err := setField(fv, val); err != nil {
			return &Error{Field: key, Err: fmt.Errorf("%w: %v", ErrParse, err)}
		}

		if err := validateEnumValue(key, fv, tag.Enum); err != nil {
			return err
		}
	}
	return nil
}

func validateRequired(cfg any) error {
	return validateRequiredAgainstValues(cfg, nil, "")
}

func validateRequiredAgainstValues(cfg any, values map[string]any, prefix string) error {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()
	return checkRequired(v, t, "", values, prefix)
}

func checkRequired(v reflect.Value, t reflect.Type, path string, values map[string]any, prefix string) error {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		tag := parseEnvxTag(field.Tag.Get("envx"))
		if tag.Skip {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			nestedName := toScreamingSnake(field.Name)
			if tag.Name != "" {
				nestedName = tag.Name
			}
			nestedPath := path + nestedName + "_"
			if err := checkRequired(fv, field.Type, nestedPath, values, prefix); err != nil {
				return err
			}
			continue
		}
		if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct && field.Type.Elem() != reflect.TypeOf(time.Time{}) {
			if fv.IsNil() {
				if tag.Required {
					key := withPrefix(prefix, fieldKey(path, field.Name, tag.Name))
					return &Error{Field: key, Err: ErrRequired}
				}
				continue
			}

			nestedName := toScreamingSnake(field.Name)
			if tag.Name != "" {
				nestedName = tag.Name
			}
			nestedPath := path + nestedName + "_"
			if err := checkRequired(fv.Elem(), field.Type.Elem(), nestedPath, values, prefix); err != nil {
				return err
			}
			continue
		}

		if tag.Required {
			key := withPrefix(prefix, fieldKey(path, field.Name, tag.Name))
			if requiredMissing(key, fv, values) {
				return &Error{Field: key, Err: ErrRequired}
			}
		}

	}
	return nil
}

func requiredMissing(key string, fv reflect.Value, values map[string]any) bool {
	if values == nil {
		return isZero(fv)
	}

	val, ok := values[key]
	if !ok || val == nil {
		return true
	}

	if isEmptyStringValue(fv) {
		return true
	}

	return false
}

func isEmptyStringValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return true
		}
		return isEmptyStringValue(v.Elem())
	}
	return v.Kind() == reflect.String && v.String() == ""
}

func validateEnumValue(key string, fv reflect.Value, allowed []string) error {
	if len(allowed) == 0 || !fv.IsValid() || fv.IsZero() {
		return nil
	}
	if fv.Kind() != reflect.String {
		return &Error{Field: key, Err: fmt.Errorf("%w: enum can only be used with string fields", ErrParse)}
	}
	val := fv.String()
	for _, option := range allowed {
		if val == option {
			return nil
		}
	}
	return &Error{Field: key, Err: fmt.Errorf("%w: %q not in enum", ErrParse, val)}
}

func logDeprecatedKey(logger Logger, key string) {
	if logger == nil {
		return
	}
	logger.Printf("envx: deprecated key used: %s\n", key)
}

func isZero(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}
	return v.IsZero()
}

func setField(fv reflect.Value, val any) error {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			fv.Set(reflect.New(fv.Type().Elem()))
		}
		return setField(fv.Elem(), val)
	}

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
		if fv.OverflowInt(v) {
			return fmt.Errorf("duration overflow: %d", v)
		}
		fv.SetInt(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return fmt.Errorf("invalid duration value: %v", val)
		}
		n := int64(v)
		if fv.OverflowInt(n) {
			return fmt.Errorf("duration overflow: %v", val)
		}
		fv.SetInt(n)
	default:
		return fmt.Errorf("invalid duration type: %T", val)
	}
	return nil
}

func setIntValue(fv reflect.Value, val any) error {
	switch v := val.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
			return fmt.Errorf("invalid int value: %v", val)
		}
		n := int64(v)
		if fv.OverflowInt(n) {
			return fmt.Errorf("invalid int value: %v", val)
		}
		fv.SetInt(n)
	case int, int8, int16, int32, int64:
		n := reflect.ValueOf(v).Int()
		if fv.OverflowInt(n) {
			return fmt.Errorf("invalid int value: %v", val)
		}
		fv.SetInt(n)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		if fv.OverflowInt(n) {
			return fmt.Errorf("invalid int value: %v", val)
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
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || math.Trunc(v) != v {
			return fmt.Errorf("invalid uint value: %v", val)
		}
		n := uint64(v)
		if float64(n) != v || fv.OverflowUint(n) {
			return fmt.Errorf("invalid uint value: %v", val)
		}
		fv.SetUint(n)
	case uint, uint8, uint16, uint32, uint64:
		n := reflect.ValueOf(v).Uint()
		if fv.OverflowUint(n) {
			return fmt.Errorf("invalid uint value: %v", val)
		}
		fv.SetUint(n)
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return err
		}
		if fv.OverflowUint(n) {
			return fmt.Errorf("invalid uint value: %v", val)
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

func fieldKey(path, fieldName, tagName string) string {
	name := toScreamingSnake(fieldName)
	if tagName != "" {
		name = tagName
	}

	if path != "" && tagName != "" && strings.HasPrefix(name, path) {
		return name
	}

	return path + name
}

func withPrefix(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "_" + key
}

func hasAnyKeyWithPrefix(values map[string]any, keyPrefix string) bool {
	for k, v := range values {
		if strings.HasPrefix(k, keyPrefix) && v != nil {
			return true
		}
	}
	return false
}
