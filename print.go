package envx

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"
)

// Print writes configuration to stdout with secret masking.
func Print[T any](cfg *T) {
	PrintTo(os.Stdout, cfg)
}

// PrintTo writes configuration to w with secret masking.
func PrintTo[T any](w io.Writer, cfg *T) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	fmt.Fprintln(w, "Configuration:")
	fmt.Fprintln(w, strings.Repeat("─", 50))
	printStruct(w, v, t, "")
	fmt.Fprintln(w, strings.Repeat("─", 50))
}

func printStruct(w io.Writer, v reflect.Value, t reflect.Type, indent string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Time{}) {
			fmt.Fprintf(w, "%s%s:\n", indent, field.Name)
			printStruct(w, fv, field.Type, indent+"  ")
			continue
		}

		name := toScreamingSnake(field.Name)
		val := fmt.Sprintf("%v", fv.Interface())

		// Mask secrets
		if isSecret(field) && len(val) > 0 {
			if len(val) > 8 {
				val = val[:3] + "***" + val[len(val)-3:]
			} else {
				val = "***"
			}
		}

		fmt.Fprintf(w, "%s%-25s = %s\n", indent, name, val)
	}
}

func isSecret(field reflect.StructField) bool {
	if field.Tag.Get("secret") == "true" {
		return true
	}
	upper := strings.ToUpper(field.Name)
	return strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "KEY")
}
