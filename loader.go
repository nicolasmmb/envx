package envx

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// Load loads configuration into T using environment variables.
// Field names are converted to SCREAMING_SNAKE_CASE for env var lookup.
func Load[T any](opts ...Option) (*T, error) {
	o := &options{output: os.Stdout}
	for _, opt := range opts {
		opt(o)
	}

	// Default providers: defaults from tags, then env
	if len(o.providers) == 0 {
		o.providers = []Provider{
			Defaults[T](),
			Env(),
		}
	}

	// Merge all providers
	values := make(map[string]string)
	for _, p := range o.providers {
		v, err := p.Values()
		if err != nil {
			return nil, err
		}
		for k, val := range v {
			values[k] = val
		}
	}

	// Parse into struct
	var cfg T
	if err := parse(&cfg, values, o.prefix); err != nil {
		return nil, err
	}

	// Validate required fields
	if err := validateRequired(&cfg); err != nil {
		return nil, err
	}

	// Custom validator
	if o.validator != nil {
		if err := o.validator(&cfg); err != nil {
			return nil, &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
		}
	}

	// Interface validator
	if v, ok := any(&cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
		}
	}

	return &cfg, nil
}

// MustLoad is like Load but panics on error.
func MustLoad[T any](opts ...Option) *T {
	cfg, err := Load[T](opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

// Loader provides hot-reloading capabilities.
type Loader[T any] struct {
	opts     []Option
	config   atomic.Pointer[T]
	version  atomic.Int64
	stop     chan struct{}
	onReload func()
}

// NewLoader creates a loader for hot-reloadable configuration.
func NewLoader[T any](opts ...Option) *Loader[T] {
	l := &Loader[T]{opts: opts}

	// Extract onReload callback
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	l.onReload = o.onReload

	return l
}

// Load loads the configuration.
func (l *Loader[T]) Load() (*T, error) {
	cfg, err := Load[T](l.opts...)
	if err != nil {
		return nil, err
	}
	l.config.Store(cfg)
	l.version.Add(1)
	return cfg, nil
}

// MustLoad loads configuration or panics.
func (l *Loader[T]) MustLoad() *T {
	cfg, err := l.Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

// Get returns the current configuration.
func (l *Loader[T]) Get() *T {
	return l.config.Load()
}

// Version returns the configuration version (increments on reload).
func (l *Loader[T]) Version() int64 {
	return l.version.Load()
}

// StartWatching starts watching for file changes.
func (l *Loader[T]) StartWatching() {
	o := &options{}
	for _, opt := range l.opts {
		opt(o)
	}

	if o.watchPath == "" {
		return
	}

	l.stop = make(chan struct{})
	var lastMod time.Time

	if info, err := os.Stat(o.watchPath); err == nil {
		lastMod = info.ModTime()
	}

	go func() {
		ticker := time.NewTicker(o.watchEvery)
		defer ticker.Stop()

		for {
			select {
			case <-l.stop:
				return
			case <-ticker.C:
				info, err := os.Stat(o.watchPath)
				if err != nil {
					continue
				}
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
					if _, err := l.Load(); err == nil && l.onReload != nil {
						l.onReload()
					}
				}
			}
		}
	}()
}

// StopWatching stops the file watcher.
func (l *Loader[T]) StopWatching() {
	if l.stop != nil {
		close(l.stop)
	}
}
