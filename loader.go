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

func LoadFromEnv[T any](opts ...Option) (*T, error) {
	withEnv := func(o *options) {
		o.providers = append([]Provider{
			DefaultsWithPrefix[T](o.prefix),
			File(".env"),
			Env(),
		}, o.providers...)
	}
	return Load[T](append(opts, withEnv)...)
}

func loadInternal[T any](opts ...Option) (map[string]any, *T, error) {
	o := prepareOptions[T](opts)

	values := make(map[string]any)
	for _, p := range o.providers {
		v, err := p.Values()
		if err != nil {
			return nil, nil, err
		}
		pa, ok := p.(prefixAware)
		if o.prefix != "" && (!ok || !pa.PrefixAware()) {
			v = applyPrefix(v, o.prefix)
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

	if err := runOptionValidator(o.validator, &cfg); err != nil {
		return nil, nil, err
	}

	if err := runTypeValidator(&cfg); err != nil {
		return nil, nil, err
	}

	return values, &cfg, nil
}

func prepareOptions[T any](opts []Option) *options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	finalizeOptions[T](o)
	return o
}

func finalizeOptions[T any](o *options) {
	if o.logger == nil {
		o.logger = newWriterLogger(os.Stdout)
	}
	if len(o.providers) == 0 {
		o.providers = []Provider{
			DefaultsWithPrefix[T](o.prefix),
			Env(),
		}
	}
}

func (l *Loader[T]) reloadConfig(o *options) {
	l.mu.Lock()
	defer l.mu.Unlock()

	oldConfig := l.config
	_, newConfig, err := loadInternal[T](l.opts...)

	if err != nil {
		l.logReloadError(o, "reload failed", err)
		return
	}

	if reflect.DeepEqual(oldConfig, newConfig) {
		return
	}

	l.config = newConfig
	l.version++
	l.triggerOnReload(oldConfig, newConfig)
}

func (l *Loader[T]) logReloadError(o *options, msg string, err error) {
	o.logger.Printf("envx: %s: %v\n", msg, err)
	if o.onReloadError != nil {
		o.onReloadError(err)
	}
}

func (l *Loader[T]) triggerOnReload(oldConfig, newConfig *T) {
	if l.onReload != nil {
		go l.onReload(oldConfig, newConfig)
	}
}

func runOptionValidator[T any](validator func(any) error, cfg *T) error {
	if validator == nil {
		return nil
	}
	return wrapValidationError(validator(cfg))
}

func runTypeValidator[T any](cfg *T) error {
	v, ok := any(cfg).(Validator)
	if !ok {
		return nil
	}
	return wrapValidationError(v.Validate())
}

func wrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Field: "config", Err: fmt.Errorf("%w: %v", ErrValidation, err)}
}

func MustLoad[T any](opts ...Option) *T {
	cfg, err := Load[T](opts...)
	if err != nil {
		panic(err)
	}
	return cfg
}

func MustLoadFromEnv[T any](opts ...Option) *T {
	cfg, err := LoadFromEnv[T](opts...)
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
	watchWG    sync.WaitGroup
	mu         sync.RWMutex
	isWatching bool
	onReload   func(any, any)
}

type prefixAware interface {
	PrefixAware() bool
}

func NewLoader[T any](opts ...Option) *Loader[T] {
	l := &Loader[T]{opts: opts}
	o := prepareOptions[T](opts)
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

	o := prepareOptions[T](l.opts)

	if o.watchPath == "" {
		return nil
	}

	if err := l.ensureConfigLoaded(o); err != nil {
		return err
	}

	if o.watchEvery <= 0 {
		err := fmt.Errorf("envx: watch interval must be greater than zero")
		o.logger.Printf("%v\n", err)
		return err
	}

	l.stop = make(chan struct{})
	l.watchWG = sync.WaitGroup{}
	l.watchWG.Add(1)
	l.isWatching = true

	watcher := newWatchLoop(l, o, os.Stat)
	go watcher.run(l.stop, &l.watchWG)

	return nil
}

type statFunc func(string) (os.FileInfo, error)

type watchLoop[T any] struct {
	loader   *Loader[T]
	opts     *options
	path     string
	interval time.Duration
	stat     statFunc
}

func newWatchLoop[T any](loader *Loader[T], opts *options, stat statFunc) watchLoop[T] {
	return watchLoop[T]{
		loader:   loader,
		opts:     opts,
		path:     opts.watchPath,
		interval: opts.watchEvery,
		stat:     stat,
	}
}

func (w watchLoop[T]) run(stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	lastMod := w.modTime()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			info, err := w.stat(w.path)
			if err != nil {
				continue
			}

			modTime := info.ModTime()
			if !modTime.After(lastMod) {
				continue
			}

			lastMod = modTime
			w.loader.reloadConfig(w.opts)
		}
	}
}

func (w watchLoop[T]) modTime() time.Time {
	info, err := w.stat(w.path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (l *Loader[T]) ensureConfigLoaded(o *options) error {
	if l.config != nil {
		return nil
	}

	if _, err := l.loadLocked(); err != nil {
		l.logReloadError(o, "watch load failed", err)
		return err
	}

	return nil
}

func (l *Loader[T]) StopWatching() {
	l.mu.Lock()
	if !l.isWatching {
		l.mu.Unlock()
		return
	}

	stop := l.stop
	wg := &l.watchWG

	l.stop = nil
	l.isWatching = false
	l.mu.Unlock()

	if stop != nil {
		close(stop)
	}

	wg.Wait()
}

func applyPrefix(values map[string]any, prefix string) map[string]any {
	if prefix == "" {
		return values
	}
	prefixed := make(map[string]any, len(values))
	for k, v := range values {
		prefixed[prefix+"_"+k] = v
	}
	return prefixed
}
