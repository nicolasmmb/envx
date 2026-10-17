// Package envx provides type-safe configuration loading for Go applications.
//
// Tag syntax:
//
//	envx:"name=NAME,required=true,secret=true,default=8080"
//	envx:"-" to ignore a field.
//
// Defaults and required checks are derived from the envx tag.
//
// Enum values are validated from the type itself by implementing Enum,
// EnumValidator or encoding.TextUnmarshaler. The enum struct tag is not supported.
package envx
