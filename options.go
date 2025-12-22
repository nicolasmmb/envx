package envx

import (
	"io"
	"path/filepath"
	"strings"
	"time"
)

type Option func(*options)

type options struct {
	providers  []Provider
	prefix     string
	output     io.Writer
	onReload   func(any, any)
	validator  func(any) error
	watchPath  string
	watchEvery time.Duration
}

func WithProvider(p Provider) Option {
	return func(o *options) {
		o.providers = append(o.providers, p)
	}
}

func WithPrefix(prefix string) Option {
	return func(o *options) {
		o.prefix = strings.ToUpper(prefix)
	}
}

func WithOutput(w io.Writer) Option {
	return func(o *options) {
		o.output = w
	}
}

func WithOnReload[T any](fn func(old *T, new *T)) Option {
	return func(o *options) {
		o.onReload = func(old any, new any) {
			oCfg, ok1 := old.(*T)
			nCfg, ok2 := new.(*T)
			if ok1 && ok2 {
				fn(oCfg, nCfg)
			}
		}
	}
}

func WithValidator(fn func(any) error) Option {
	return func(o *options) {
		o.validator = fn
	}
}

func WithWatch(path string, interval time.Duration) Option {
	return func(o *options) {
		o.watchPath, _ = filepath.Abs(path)
		o.watchEvery = interval
	}
}
