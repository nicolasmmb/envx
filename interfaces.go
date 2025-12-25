package envx

type Provider interface {
	Values() (map[string]any, error)
}

type Validator interface {
	Validate() error
}
