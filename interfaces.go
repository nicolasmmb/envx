package envx

type Provider interface {
	Values() (map[string]any, error)
}

type Validator interface {
	Validate() error
}

// Enum is implemented by enum types that declare the values they accept.
// envx validates loaded values against EnumValues; enum values are not declared by tags.
type Enum interface {
	EnumValues() []string
}

// EnumValidator is implemented by native enum types that validate their own value.
// envx calls IsValid after loading a value into the field.
type EnumValidator interface {
	IsValid() bool
}
