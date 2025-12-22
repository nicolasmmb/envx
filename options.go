package envx

import (
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Option configures the loader.
type Option func(*options)

type options struct {
	providers  []Provider
	prefix     string
	output     io.Writer
	onReload   func()
	validator  func(any) error
	watchPath  string
	watchEvery time.Duration
}

// WithProvider adds a configuration provider.
func WithProvider(p Provider) Option {
	return func(o *options) {
		o.providers = append(o.providers, p)
	}
}

// WithPrefix sets an environment variable prefix.
// Example: WithPrefix("APP") makes "Port" look for "APP_PORT".
func WithPrefix(prefix string) Option {
	return func(o *options) {
		o.prefix = strings.ToUpper(prefix)
	}
}

// WithOutput sets where Print() writes to (default: os.Stdout).
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		o.output = w
	}
}

// WithOnReload sets a callback for configuration reloads.
func WithOnReload(fn func()) Option {
	return func(o *options) {
		o.onReload = fn
	}
}

// WithValidator sets a custom validation function.
func WithValidator(fn func(any) error) Option {
	return func(o *options) {
		o.validator = fn
	}
}

// WithWatch enables hot-reloading from a file.
func WithWatch(path string, interval time.Duration) Option {
	return func(o *options) {
		o.watchPath, _ = filepath.Abs(path)
		o.watchEvery = interval
	}
}
