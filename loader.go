package envx

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"
)

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
			DefaultsWithPrefix[T](o.prefix),
			Env(),
		}
	}

	values := make(map[string]any)
	for _, p := range o.providers {
		if pa, ok := p.(interface{ setPrefix(string) }); ok {
			pa.setPrefix(o.prefix)
		}
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

func MustLoad[T any](opts ...Option) *T {
	cfg, err := Load[T](opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

type Loader[T any] struct {
	opts       []Option
	config     *T
	version    int64
	stop       chan struct{}
	watchWG    *sync.WaitGroup
	mu         sync.RWMutex
	isWatching bool
	onReload   func(any, any)
}

func NewLoader[T any](opts ...Option) *Loader[T] {
	l := &Loader[T]{opts: opts}
	o := &options{output: os.Stdout}
	for _, opt := range opts {
		opt(o)
	}
	l.onReload = o.onReload
	return l
}

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

func (l *Loader[T]) MustLoad() *T {
	cfg, err := l.Load()
	if err != nil {
		panic(err)
	}
	return cfg
}

func (l *Loader[T]) Get() *T {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.config
}

func (l *Loader[T]) Version() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.version
}

func (l *Loader[T]) StartWatching() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isWatching {
		return nil
	}

	o := &options{output: os.Stdout}
	for _, opt := range l.opts {
		opt(o)
	}

	if o.output == nil {
		o.output = os.Stdout
	}

	if o.watchPath == "" {
		return nil
	}

	if l.config == nil {
		if _, err := l.loadLocked(); err != nil {
			fmt.Fprintf(o.output, "envx: watch load failed: %v\n", err)
			return err
		}
	}

	if o.watchEvery <= 0 {
		err := fmt.Errorf("envx: watch interval must be greater than zero")
		fmt.Fprintln(o.output, err.Error())
		return err
	}

	l.stop = make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	l.watchWG = wg
	stop := l.stop
	l.isWatching = true
	var lastMod time.Time

	if info, err := os.Stat(o.watchPath); err == nil {
		lastMod = info.ModTime()
	}

	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		ticker := time.NewTicker(o.watchEvery)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
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

					if err != nil {
						fmt.Fprintf(o.output, "envx: reload failed: %v\n", err)
						l.mu.Unlock()
						continue
					}

					changed := !reflect.DeepEqual(oldConfig, newConfig)

					if changed {
						l.config = newConfig
						l.version++
						if l.onReload != nil {

							go l.onReload(oldConfig, newConfig)
						}
					}
					l.mu.Unlock()
				}
			}
		}
	}(wg)

	return nil
}

func (l *Loader[T]) StopWatching() {
	l.mu.Lock()
	if !l.isWatching {
		l.mu.Unlock()
		return
	}

	stop := l.stop
	wg := l.watchWG

	l.stop = nil
	l.isWatching = false
	l.watchWG = nil
	l.mu.Unlock()

	if stop != nil {
		close(stop)
	}

	if wg != nil {
		wg.Wait()
	}
}
