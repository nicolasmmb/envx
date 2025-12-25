package envx

// KeyMapper allows customizing how struct field names are mapped to keys.
type KeyMapper interface {
	Field(string) string
}

type screamingSnakeMapper struct{}

func (screamingSnakeMapper) Field(name string) string {
	return toScreamingSnake(name)
}

var defaultMapper screamingSnakeMapper
