// Package envx provides type-safe configuration loading from environment variables.
//
// Field names map directly to environment variable names. Use struct tags for defaults,
// validation, and secret masking.
//
// Basic usage:
//
//	type Config struct {
//	    Port        int    `default:"8080"`
//	    DatabaseURL string `required:"true"`
//	}
//
//	cfg, err := envx.Load[Config]()
//
// The field "DatabaseURL" loads from env var "DATABASE_URL" (auto-converted to SCREAMING_SNAKE_CASE).
//
// # Struct Tags
//
// The following struct tags are supported:
//
//   - default:"value" - Default value if env var is not set
//   - required:"true" - Field must have a non-zero value
//   - secret:"true"   - Mask value in Print output
//
// # Providers
//
// Configuration values can come from multiple sources (providers):
//
//   - Env()         - Environment variables
//   - Defaults[T]() - Default values from struct tags
//   - File(path)    - JSON or .env configuration file
//   - Map(values)   - Explicit key-value map
//
// # Combining Providers
//
// To combine a .env file with system environment variables:
//
//	envx.Load[Config](
//	    envx.WithProvider(envx.Defaults[Config]()),
//	    envx.WithProvider(envx.File(".env")), // .env overrides defaults
//	    envx.WithProvider(envx.Env()),         // System env overrides .env
//	)
//
// # Hot Reloading
//
// Use NewLoader for hot-reloadable configuration:
//
//	loader := envx.NewLoader[Config](
//	    envx.WithWatch("config.json", 5*time.Second),
//	    envx.WithOnReload(func() { log.Println("config reloaded") }),
//	)
//
//	cfg := loader.MustLoad()
//	loader.StartWatching()
//	defer loader.StopWatching()
package envx
