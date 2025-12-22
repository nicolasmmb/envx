package envx

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Load loads configuration into T using environment variables.
func Load[T any](opts ...Option) (*T, error) {
	o := &options{output: os.Stdout}
	for _, opt := range opts {
		opt(o)
	}

	if len(o.providers) == 0 {
		o.providers = []Provider{
			Defaults[T](),
			Env(),
		}
	}

	values := make(map[string]any)
	for _, p := range o.providers {
		v, err := p.Values()
		if err != nil {
			return nil, err
		}
		for k, val := range v {
			values[k] = val
		}
	}

	var cfg T
	if err := parse(&cfg, values, o.prefix); err != nil {
		return nil, err
	}

	if err := validateRequired(&cfg); err != nil {
		return nil, err
	}

	if o.validator != nil {
		if err := o.validator(&cfg); err != nil {
			return nil, &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
		}
	}

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
	opts       []Option
	config     *T
	version    int64
	stop       chan struct{}
	mu         sync.RWMutex // Protects config, version, stop, isWatching
	isWatching bool
	onReload   func()
}

// NewLoader creates a loader for hot-reloadable configuration.
func NewLoader[T any](opts ...Option) *Loader[T] {
	l := &Loader[T]{opts: opts}
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

	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = cfg
	l.version++
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
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

// Version returns the configuration version.
func (l *Loader[T]) Version() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.version
}

// StartWatching starts watching for file changes.
func (l *Loader[T]) StartWatching() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isWatching {
		return
	}

	o := &options{}
	for _, opt := range l.opts {
		opt(o)
	}

	if o.watchPath == "" {
		return
	}

	l.stop = make(chan struct{})
	l.isWatching = true
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
					// Load needs lock, but we call it from outside the lock loop
					// Load itself acquires lock. Safe.
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
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isWatching {
		return
	}

	if l.stop != nil {
		close(l.stop)
		l.stop = nil
	}
	l.isWatching = false
}
