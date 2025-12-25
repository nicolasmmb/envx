package envx

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Option func(*options)

type options struct {
	providers     []Provider
	prefix        string
	logger        Logger
	onReload      func(any, any)
	onReloadError func(error)
	validator     func(any) error
	watchPath     string
	watchEvery    time.Duration
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
		o.logger = newWriterLogger(w)
	}
}

func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
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

func WithOnReloadError(fn func(error)) Option {
	return func(o *options) {
		o.onReloadError = fn
	}
}

func WithValidator[T any](fn func(*T) error) Option {
	return func(o *options) {
		o.validator = func(cfg any) error {
			c, ok := cfg.(*T)
			if !ok {
				return fmt.Errorf("%w: validator type mismatch", ErrUnsupportedType)
			}
			return fn(c)
		}
	}
}

func WithWatch(path string, interval time.Duration) Option {
	return func(o *options) {
		o.watchPath, _ = filepath.Abs(path)
		o.watchEvery = interval
	}
}

func defaultOptions() *options {
	return &options{
		logger: newWriterLogger(os.Stdout),
	}
}
