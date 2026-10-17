<div align="center">

# ⚙️ envx

### Type-safe configuration for Go applications

[![Go Reference](https://pkg.go.dev/badge/github.com/nicolasmmb/envx.svg)](https://pkg.go.dev/github.com/nicolasmmb/envx)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasmmb/envx)](https://goreportcard.com/report/github.com/nicolasmmb/envx)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/nicolasmmb/envx?sort=semver&display_name=tag)](https://github.com/nicolasmmb/envx/releases)

**Zero dependencies • Type-safe • Simple API • Hot reload**

[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Documentation](#-documentation) •
[Examples](#-examples)

---

</div>

## ✨ Why envx?

```go
type Config struct {
    Port        int           `envx:"name=PORT,default=8080"`
    DatabaseURL string        `envx:"name=DATABASE_URL,required=true"`
    JWTSecret   string        `envx:"name=JWT_SECRET,secret=true"`
    Timeout     time.Duration `envx:"name=TIMEOUT,default=30s"`
}

cfg := envx.MustLoad[Config]()
```

| Field | Environment Variable |
|-------|---------------------|
| `Port` | `PORT` |
| `DatabaseURL` | `DATABASE_URL` |
| `JWTSecret` | `JWT_SECRET` |
| `Timeout` | `TIMEOUT` |

**That's it.** No boilerplate. No manual parsing. Just define your struct and go.

---

## 🎯 Features

<table>
<tr>
<td width="50%">

### 🚀 Zero Dependencies
Only Go standard library. No external packages.

### 🔒 Type-Safe
Full type safety with Go 1.21+ generics.

### 🐍 Auto Naming
`CamelCase` → `SCREAMING_SNAKE_CASE` automatically.

</td>
<td width="50%">

### ✅ Validation
Required fields, custom validators, and native enums.

### 🔐 Secret Masking
Auto-mask sensitive values in logs.

### 🔄 Hot Reload
Watch files and reload on changes.

</td>
</tr>
</table>

---

## 📦 Installation

```bash
go get github.com/nicolasmmb/envx
```

---

## 🚀 Quick Start

```go
package main

import (
    "fmt"
    "github.com/nicolasmmb/envx"
)

type Config struct {
    Port        int    `envx:"name=PORT,default=8080"`
    DatabaseURL string `envx:"name=DATABASE_URL,required=true"`
    Debug       bool   `envx:"name=DEBUG,default=false"`
}

func main() {
    cfg, err := envx.Load[Config]() // defaults + .env + environment
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("🚀 Server starting on port %d\n", cfg.Port)
}
```

```bash
export DATABASE_URL="postgres://localhost/mydb"
go run main.go
# 🚀 Server starting on port 8080
```

---

## 📖 Documentation

## ✅ Common Recipes

### 1) Defaults + .env + Environment (most common)

```go
type Config struct {
    Port        int    `envx:"name=PORT,default=8080"`
    DatabaseURL string `envx:"name=DATABASE_URL,required=true"`
}

cfg := envx.MustLoad[Config]()
```

### 2) JSON file + Environment

```go
type Config struct {
    Port int `envx:"name=PORT,default=8080"`
}

cfg := envx.MustLoad[Config](
    envx.WithProvider(envx.File("config.json")),
)
```

### 3) Prefix for Multi-App Envs

```go
type Config struct {
    Port int `envx:"name=PORT,default=8080"`
}

cfg := envx.MustLoad[Config](
    envx.WithPrefix("API"),
)
// reads API_PORT, API_DATABASE_URL, etc.
```

### 4) Validation (type-safe)

```go
type Config struct {
    Port int `envx:"name=PORT,default=8080"`
}

cfg := envx.MustLoad[Config](
    envx.WithValidator(func(cfg *Config) error {
        if cfg.Port < 1024 {
            return fmt.Errorf("port must be >= 1024")
        }
        return nil
    }),
)
```

### 5) Hot Reload

```go
loader := envx.NewLoader[Config](
    envx.WithProvider(envx.File("config.json")),
    envx.WithWatch("config.json", 5*time.Second),
    envx.WithOnReload(func(old *Config, new *Config) {
        log.Printf("reloaded: %d -> %d", old.Port, new.Port)
    }),
)

cfg, err := loader.Load()
if err != nil {
    log.Fatal(err)
}
_ = loader.StartWatching()
defer loader.StopWatching()
```

### 6) Complete End-to-End Setup

See `example/complete/main.go` for a full setup with:
- prefix + defaults + `.env` + environment
- custom provider + precedence mode
- type-safe validator
- native enum with `EnumValues`
- hot reload callbacks (`OnReload` + `OnReloadError`)

### Struct Tags

| Tag | Description | Example |
|:----|:------------|:--------|
| `envx` | Unified config tag | `envx:"name=PORT,required=true,secret=true,default=8080"` |

Use `envx:"-"` to ignore a field.
Use `deprecated=true` to log a warning when a key is used.
Enum values are declared by the type, not by tags. See [Enums](#enums).

### Enums

envx validates enum values from the type itself. The `enum=` struct tag is no
longer supported and is ignored: declare the accepted values on the enum type and
keep the `envx` tag only for naming, defaults, required checks, and so on.

Go does not expose package constants to reflection, so an enum type must expose
its values through one of these contracts:

| Contract | Use when | envx behavior |
|:---------|:---------|:--------------|
| `EnumValues() []string` | String-backed enums | Checks the loaded value against the returned list |
| `IsValid() bool` | Any underlying kind, especially integer enums | Calls `IsValid` after parsing |
| `encoding.TextUnmarshaler` | Generated enums or custom parsing | Decodes through `UnmarshalText`; the method rejects invalid values |

#### Option 1: `EnumValues` (recommended for string enums)

```go
type Environment string

const (
    Development Environment = "development"
    Staging     Environment = "staging"
    Production  Environment = "production"
)

// EnumValues declares the accepted values.
func (Environment) EnumValues() []string {
    return []string{
        string(Development),
        string(Staging),
        string(Production),
    }
}

type Config struct {
    Env Environment `envx:"name=APP_ENV,default=development"`
}
```

Invalid values fail with `ErrParse`:

```text
envx: APP_ENV: parse error: "broken" not in enum (development, staging, production)
```

#### Option 2: `IsValid` (any enum type)

Implement `IsValid() bool` when the type validates itself. This works for any
underlying kind, including integer enums:

```go
type LogLevel int

const (
    Debug LogLevel = iota
    Info
    Warn
    Error
)

func (l LogLevel) IsValid() bool {
    switch l {
    case Debug, Info, Warn, Error:
        return true
    default:
        return false
    }
}

type Config struct {
    LogLevel LogLevel `envx:"name=LOG_LEVEL,default=1"` // LOG_LEVEL=2
}
```

`EnumValues` is checked first when a type implements both. For integer enums,
`IsValid` receives the parsed numeric value. To accept names such as
`LOG_LEVEL=info`, use `encoding.TextUnmarshaler`.

#### Option 3: `encoding.TextUnmarshaler`

Types implementing `encoding.TextUnmarshaler` are decoded through
`UnmarshalText`. This is useful for enums generated by tools such as `go-enum`
or `enumer`, and for mapping names to integer values:

```go
type LogLevel int

const (
    Debug LogLevel = iota
    Info
    Warn
    Error
)

func (l *LogLevel) UnmarshalText(text []byte) error {
    switch string(text) {
    case "debug":
        *l = Debug
    case "info":
        *l = Info
    case "warn":
        *l = Warn
    case "error":
        *l = Error
    default:
        return fmt.Errorf("invalid log level %q", text)
    }
    return nil
}
```

Errors returned by `UnmarshalText` are wrapped as `ErrParse`, with the config
key in the error message.

#### Enum fields and validation

Native enums work with defaults, `.env`, environment variables, JSON files, and
custom providers. They also work with pointer and slice fields:

```go
type Config struct {
    Env    Environment   `envx:"name=APP_ENV,default=development"`
    Others []Environment `envx:"name=OTHER_ENVS,default=development,staging"`
}
```

- A field with no value is left at its zero value and is not validated.
- Defaults are validated too.
- Slice elements report their index in errors, for example
  `envx: OTHER_ENVS[1]: parse error: "broken" not in enum (...)`.
- `EnumValues` entries are compared against the field's formatted value, so a
  string-backed enum should return the same strings used by its constants.
- Value and pointer receivers are both supported for all three contracts.

### Supported Types

| Type | Example Value |
|:-----|:--------------|
| `string` | `"hello"` |
| `int`, `int64` | `42` |
| `float64` | `3.14` |
| `bool` | `true`, `false` |
| `time.Duration` | `30s`, `5m`, `1h` |
| `[]string` | `a,b,c` |
| Native enums | Implement `Enum`, `EnumValidator` or `encoding.TextUnmarshaler` |
| Nested structs | See below |

### Nested Structs

```go
type Config struct {
    Server struct {
        Host string `envx:"name=HOST,default=0.0.0.0"`
        Port int    `envx:"name=PORT,default=8080"`
    }
    Database struct {
        URL      string `envx:"name=DATABASE_URL,required=true"`
        PoolSize int    `envx:"name=POOL_SIZE,default=10"`
    }
}
```

```bash
export SERVER_HOST="localhost"
export SERVER_PORT="3000"
export DATABASE_URL="postgres://localhost/db"
export DATABASE_POOL_SIZE="20"
```

---

## 🔧 Advanced Usage

### Environment Prefix

```go
cfg, _ := envx.Load[Config](
    envx.WithPrefix("MYAPP"),
)
// Port → MYAPP_PORT
// DatabaseURL → MYAPP_DATABASE_URL

> Prefix is strict: when set, only prefixed variables are considered (defaults are automatically mapped with the prefix).
```

### Multiple Sources

```go
cfg, _ := envx.Load[Config](                   // Defaults + .env + environment
    envx.WithProvider(envx.File("config.json")), // optional file
)
```

> `Load` gives you a conventional stack: struct defaults → `.env` → environment (highest priority).

### Precedence Control

```go
cfg, _ := envx.Load[Config](
    envx.WithPrecedence(envx.PrecedenceCustomWins),
    envx.WithProvider(envx.Map(map[string]string{
        "PORT": "9000",
    })),
)
```

By default, `PrecedenceEnvWins` is used. Set `PrecedenceCustomWins` when custom providers should override environment variables.

### Custom Validation

```go
cfg, err := envx.Load[Config](
    envx.WithValidator(func(cfg *Config) error {
        if cfg.Port < 1024 {
            return errors.New("port must be >= 1024")
        }
        return nil
    }),
)
```

### Hot Reload

```go
loader := envx.NewLoader[Config](
    envx.WithProvider(envx.File("config.json")),
    envx.WithWatch("config.json", 5*time.Second),
    envx.WithOnReload(func(old *Config, new *Config) {
        log.Printf("⚡ Config reloaded: %d -> %d", old.Port, new.Port)
    }),
)

cfg, err := loader.Load()
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}
if err := loader.StartWatching(); err != nil {
    log.Fatalf("failed to watch config: %v", err)
}
defer loader.StopWatching()

// Get latest config anytime
current := loader.Get()
```

### Custom Provider

```go
type VaultProvider struct {
    Address string
}

func (p *VaultProvider) Values() (map[string]any, error) {
    // Fetch from Vault, AWS SSM, etc.
    return map[string]any{
        "JWT_SECRET": "secret-from-vault",
    }, nil
}

cfg, _ := envx.Load[Config](
    envx.WithProvider(&VaultProvider{Address: "vault:8200"}),
)
```

---

## 🖨️ Printing Config

```go
envx.Print(cfg)
```

```
Configuration:
──────────────────────────────────────────────────
PORT                      = 8080
DATABASE_URL              = postgres://localhost/db
JWT_SECRET                = abc***xyz
DEBUG                     = false
──────────────────────────────────────────────────
```

> 🔐 Secrets are automatically masked based on field name or `envx:"name=JWT_SECRET,secret=true"` tag.

---

## 📁 JSON Config File

`config.json`:
```json
{
  "port": 3000,
  "databaseURL": "postgres://prod/db",
  "server": {
    "host": "0.0.0.0",
    "port": 443
  }
}
```

---

## 🧪 Examples

<details>
<summary><b>Web Server Configuration</b></summary>

```go
type Config struct {
    Server struct {
        Host         string        `envx:"name=HOST,default=0.0.0.0"`
        Port         int           `envx:"name=PORT,default=8080"`
        ReadTimeout  time.Duration `envx:"name=READ_TIMEOUT,default=5s"`
        WriteTimeout time.Duration `envx:"name=WRITE_TIMEOUT,default=10s"`
    }
    Database struct {
        URL         string `envx:"name=DATABASE_URL,required=true,secret=true"`
        MaxConns    int    `envx:"name=MAX_CONNS,default=25"`
        MaxIdleTime time.Duration `envx:"name=MAX_IDLE_TIME,default=5m"`
    }
    Auth struct {
        JWTSecret     string        `envx:"name=JWT_SECRET,required=true,secret=true"`
        TokenExpiry   time.Duration `envx:"name=TOKEN_EXPIRY,default=24h"`
        RefreshExpiry time.Duration `envx:"name=REFRESH_EXPIRY,default=168h"`
    }
    Features struct {
        Debug      bool     `envx:"name=DEBUG,default=false"`
        CORSOrigins []string `envx:"name=CORS_ORIGINS,default=http://localhost:3000"`
    }
}

func main() {
    cfg := envx.MustLoad[Config]()
    envx.Print(cfg)
    
    // Use cfg.Server.Port, cfg.Database.URL, etc.
}
```

</details>

<details>
<summary><b>Vault Provider</b></summary>

See `example/vault/main.go`:

```go
type appEnv string

const (
    AppEnvDev     appEnv = "dev"
    AppEnvStaging appEnv = "staging"
    AppEnvProd    appEnv = "prod"
)

func (appEnv) EnumValues() []string {
    return []string{"dev", "staging", "prod"}
}

type Config struct {
    DatabaseURL string `envx:"name=DATABASE_URL,required=true"`
    AppEnv      appEnv `envx:"name=APP_ENV"`
    LegacyToken string `envx:"name=LEGACY_TOKEN,deprecated=true"`
}

cfg, _ := envx.Load[Config](
    envx.WithProvider(&vaultProvider{client: client, path: "app/config"}),
)
```
</details>

<details>
<summary><b>With Validation</b></summary>

```go
type Config struct {
    Port     int    `envx:"name=PORT,default=8080"`
    LogLevel string `envx:"name=LOG_LEVEL,default=info"`
}

func (c *Config) Validate() error {
    if c.Port < 1 || c.Port > 65535 {
        return fmt.Errorf("invalid port: %d", c.Port)
    }
    
    validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
    if !validLevels[c.LogLevel] {
        return fmt.Errorf("invalid log level: %s", c.LogLevel)
    }
    
    return nil
}

func main() {
    cfg, err := envx.Load[Config]()
    if err != nil {
        log.Fatalf("Config error: %v", err)
    }
}
```

</details>

<details>
<summary><b>Multiple Environments</b></summary>

```go
func loadConfig() *Config {
    env := os.Getenv("APP_ENV")
    
    loader := envx.NewLoader[Config]()
    
    // Load environment-specific file
    switch env {
    case "production":
        loader = envx.NewLoader[Config](
            envx.WithProvider(envx.File("config.prod.json")),
        )
    case "staging":
        loader = envx.NewLoader[Config](
            envx.WithProvider(envx.File("config.staging.json")),
        )
    default:
        loader = envx.NewLoader[Config](
            envx.WithProvider(envx.File("config.local.json")),
        )
    }

    cfg, err := loader.Load()
    if err != nil {
        log.Fatalf("config load failed: %v", err)
    }
    return cfg
}
```

</details>

<details>
<summary><b>Complete Example (prefix + precedence + reload)</b></summary>

See `example/complete/main.go` and `example/complete/config.json`:

```bash
APP_DATABASE_URL=postgres://localhost/envx_complete go run ./example/complete
```

The example shows:
- default stack (`defaults -> .env -> env`)
- `WithPrecedence(...)` for custom provider ordering
- validation + reload callbacks + graceful shutdown

</details>

<details>
<summary><b>Full Feature Demo (prefix, validators, watch)</b></summary>

See `example/full/main.go`:

```go
loader := envx.NewLoader[Config](
    envx.WithPrefix("APP"),                       // strict prefix
    envx.WithProvider(envx.File("config.json")),                // optional JSON/.env
    envx.WithValidator(func(cfg *Config) error { // type-safe validator
        if cfg.App.Port < 1024 {
            return fmt.Errorf("APP_PORT must be >= 1024")
        }
        return nil
    }),
    envx.WithOnReload(func(old *Config, new *Config) {
        log.Printf("config reloaded: port %d -> %d", old.App.Port, new.App.Port)
    }),
    envx.WithWatch("config.json", 2*time.Second), // hot reload
)

cfg, err := loader.Load()
if err != nil {
    log.Fatalf("failed to load config: %v", err)
}
envx.Print(cfg)

if err := loader.StartWatching(); err != nil {
    log.Fatalf("failed to watch: %v", err)
}
defer loader.StopWatching()
```

Run it:
```bash
APP_DATABASE_URL=postgres://db/prod go run ./example/full
```

</details>

> ℹ️ When a prefix is set, only prefixed environment variables are read; defaults are auto-prefixed internally so they keep working with `WithPrefix`.

---

## 📚 API Reference

### Load Functions

```go
cfg, err := envx.Load[T](opts...)    // Load with error
cfg := envx.MustLoad[T](opts...)      // Load or panic
```

> ℹ️ `T` must be a struct type; passing primitives or pointer types returns `ErrUnsupportedType`.

### Options

```go
envx.WithPrefix(prefix)        // Env var prefix
envx.WithProvider(p)           // Add provider
envx.WithPrecedence(mode)      // Precedence between env and custom providers
envx.WithValidator(fn)         // Custom validator (type-safe)
envx.WithWatch(path, interval) // File watching
envx.WithOnReload(fn)          // Reload callback
envx.WithOnReloadError(fn)     // Reload error callback
envx.WithLogger(logger)        // Custom logger (implements Printf)
envx.WithOutput(w)             // Convenience to log to a writer
```

```go
envx.PrecedenceEnvWins
envx.PrecedenceCustomWins
```

> 🔁 File watching starts only when the initial load succeeds and the interval is greater than zero.

### Providers

```go
envx.Defaults[T]()             // Struct tag defaults
envx.Env()                     // Environment variables
envx.File(path)                // JSON or .env file
envx.Map(m)                    // String map
```

### Loader (Hot Reload)

```go
loader := envx.NewLoader[T](opts...)
loader.Load()          // Load config
loader.Get()           // Get current config
loader.Version()       // Get version number
loader.StartWatching() // Start file watcher (returns error)
loader.StopWatching()  // Stop file watcher
```

### Errors

```go
envx.ErrRequired        // Required field empty
envx.ErrValidation      // Validation failed
envx.ErrParse           // Parse error
envx.ErrUnsupportedType // Unsupported type
```

---

## 📦 Versioning

envx follows [Semantic Versioning](https://semver.org/). Releases are generated
by [release-please](https://github.com/googleapis/release-please) from
[Conventional Commits](https://www.conventionalcommits.org/) merged into `main`:

| Commit | Release |
|--------|---------|
| `feat: ...` | minor (`1.1.0`) |
| `fix: ...`, `refactor: ...`, `perf: ...` | patch (`1.0.1`) |
| `docs: ...`, `chore: ...`, `test: ...`, `ci: ...` | none |

A release PR bumps `version.go`; merging it creates the tag and the GitHub
release. Breaking changes (`feat!:`) require a new major version and a `/v2`
module path, so the automation never bumps the major on its own.

```go
fmt.Println(envx.Version) // "1.0.0"
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing`)
5. Open a Pull Request

---

## 📄 License

MIT License - see the [LICENSE](LICENSE) file for details.

---

<div align="center">

⭐ Star this repo if you find it useful!

</div>
