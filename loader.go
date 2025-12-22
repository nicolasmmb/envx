package envx

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"
)

// Load loads configuration into T using environment variables.
func Load[T any](opts ...Option) (*T, error) {
	_, cfg, err := loadInternal[T](opts...)
	return cfg, err
}

func loadInternal[T any](opts ...Option) (map[string]any, *T, error) {
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
			return nil, nil, err
		}
		for k, val := range v {
			values[k] = val
		}
	}

	var cfg T
	if err := parse(&cfg, values, o.prefix); err != nil {
		return nil, nil, err
	}

	if err := validateRequired(&cfg); err != nil {
		return nil, nil, err
	}

	if o.validator != nil {
		if err := o.validator(&cfg); err != nil {
			return nil, nil, &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
		}
	}

	if v, ok := any(&cfg).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, nil, &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
		}
	}

	return values, &cfg, nil
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
	mu         sync.RWMutex
	isWatching bool
	onReload   func(any, any)
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
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadLocked()
}

func (l *Loader[T]) loadLocked() (*T, error) {
	_, cfg, err := loadInternal[T](l.opts...)
	if err != nil {
		return nil, err
	}

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

	// Initial load to ensure we have a base config
	if l.config == nil {
		if _, err := l.loadLocked(); err != nil {
			// ignore error on initial load attempt in background?
		}
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

					l.mu.Lock()
					oldConfig := l.config
					_, newConfig, err := loadInternal[T](l.opts...)

					if err == nil {
						// Only notify/update if something actually changed deep inside?
						// Or just notify always on file change?
						// Usually file mod implies change.
						// Checking DeepEqual prevents spurious updates if file touched but content same.
						changed := !reflect.DeepEqual(oldConfig, newConfig)

						if changed {
							l.config = newConfig
							l.version++
							if l.onReload != nil {
								// Call callback in goroutine to avoid blocking/deadlock?
								// Since we hold lock, calling user code is risky.
								// But capturing old/new is fine.
								go l.onReload(oldConfig, newConfig)
							}
						}
					}
					l.mu.Unlock()
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
