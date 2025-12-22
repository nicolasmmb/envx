package envx

// Provider is a source of configuration values.
type Provider interface {
	// Values returns key-value pairs.
	Values() (map[string]any, error)
}

// Validator is implemented by config structs that validate themselves.
type Validator interface {
	Validate() error
}
