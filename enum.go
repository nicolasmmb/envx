package envx

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"time"
)

var (
	timeType                 = reflect.TypeOf(time.Time{})
	enumInterface            = reflect.TypeOf((*Enum)(nil)).Elem()
	enumValidatorInterface   = reflect.TypeOf((*EnumValidator)(nil)).Elem()
	textUnmarshalerInterface = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// validateNativeEnum validates a field against the enum contracts implemented
// by its type. Slices, arrays and pointers are validated recursively.
func validateNativeEnum(key string, fv reflect.Value) error {
	if !fv.IsValid() {
		return nil
	}

	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil
		}
		return validateNativeEnum(key, fv.Elem())
	}

	if fv.Kind() == reflect.Slice || fv.Kind() == reflect.Array {
		for i := 0; i < fv.Len(); i++ {
			if err := validateNativeEnum(fmt.Sprintf("%s[%d]", key, i), fv.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}

	if !typeImplements(fv.Type(), enumInterface) && !typeImplements(fv.Type(), enumValidatorInterface) {
		return nil
	}

	if raw, ok := lookupInterface(fv, enumInterface); ok {
		allowed := raw.(Enum).EnumValues()
		if len(allowed) > 0 {
			got := formatEnumValue(fv)
			for _, option := range allowed {
				if got == option {
					return nil
				}
			}
			return &Error{Field: key, Err: fmt.Errorf("%w: %q not in enum (%s)", ErrParse, got, strings.Join(allowed, ", "))}
		}
	}

	if raw, ok := lookupInterface(fv, enumValidatorInterface); ok {
		if !raw.(EnumValidator).IsValid() {
			return &Error{Field: key, Err: fmt.Errorf("%w: %q is not a valid %s", ErrParse, formatEnumValue(fv), fv.Type())}
		}
	}

	return nil
}

// setFromTextUnmarshaler decodes val through encoding.TextUnmarshaler when the
// field type (or a pointer to it) implements the interface.
func setFromTextUnmarshaler(fv reflect.Value, val any) (bool, error) {
	if !typeImplements(fv.Type(), textUnmarshalerInterface) {
		return false, nil
	}

	raw, ok := lookupInterface(fv, textUnmarshalerInterface)
	if !ok {
		return false, nil
	}

	if err := raw.(encoding.TextUnmarshaler).UnmarshalText([]byte(valueAsText(val))); err != nil {
		return true, err
	}
	return true, nil
}

func valueAsText(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// lookupInterface returns the field value as the given interface, checking both
// the addressable value and its address so value and pointer receivers work.
func lookupInterface(fv reflect.Value, iface reflect.Type) (any, bool) {
	if !fv.IsValid() {
		return nil, false
	}

	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return nil, false
		}
		if fv.Type().Implements(iface) && fv.CanInterface() {
			return fv.Interface(), true
		}
		return lookupInterface(fv.Elem(), iface)
	}

	if fv.CanAddr() && fv.Addr().CanInterface() && fv.Addr().Type().Implements(iface) {
		return fv.Addr().Interface(), true
	}
	if fv.CanInterface() && fv.Type().Implements(iface) {
		return fv.Interface(), true
	}

	return nil, false
}

func formatEnumValue(fv reflect.Value) string {
	if fv.Kind() == reflect.Pointer {
		if fv.IsNil() {
			return ""
		}
		return formatEnumValue(fv.Elem())
	}
	if fv.CanInterface() {
		return fmt.Sprintf("%v", fv.Interface())
	}
	return ""
}

// isLeafType reports whether t must be parsed as a scalar value instead of
// being traversed as a nested struct.
func isLeafType(t reflect.Type) bool {
	if t == timeType {
		return true
	}
	return typeImplements(t, textUnmarshalerInterface)
}

func typeImplements(t reflect.Type, iface reflect.Type) bool {
	if t.Implements(iface) {
		return true
	}
	if t.Kind() != reflect.Pointer {
		return reflect.PointerTo(t).Implements(iface)
	}
	return false
}

func isNestedStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct && !isLeafType(t)
}

func isNestedStructPointer(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && t.Elem().Kind() == reflect.Struct && !isLeafType(t.Elem())
}
