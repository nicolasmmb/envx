<div align="center">

# ⚙️ envconfig

### Type-safe configuration for Go applications

[![Go Reference](https://pkg.go.dev/badge/github.com/nicolasmmb/envconfig.svg)](https://pkg.go.dev/github.com/nicolasmmb/envconfig)
[![Go Report Card](https://goreportcard.com/badge/github.com/nicolasmmb/envconfig)](https://goreportcard.com/report/github.com/nicolasmmb/envconfig)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)

**Zero dependencies • Type-safe • Simple API • Hot reload**

[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Documentation](#-documentation) •
[Examples](#-examples)

---

</div>

## ✨ Why envconfig?

```go
type Config struct {
    Port        int           `default:"8080"`
    DatabaseURL string        `required:"true"`
    JWTSecret   string        `secret:"true"`
    Timeout     time.Duration `default:"30s"`
}

cfg := envconfig.MustLoad[Config]()
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
Required fields and custom validators.

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
go get github.com/nicolasmmb/envconfig
```

**Requirements:** Go 1.21 or later

---

## 🚀 Quick Start

```go
package main

import (
    "fmt"
    "github.com/nicolasmmb/envconfig"
)

type Config struct {
    Port        int    `default:"8080"`
    DatabaseURL string `required:"true"`
    Debug       bool   `default:"false"`
}

func main() {
    cfg, err := envconfig.Load[Config]()
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

### Struct Tags

| Tag | Description | Example |
|:----|:------------|:--------|
| `default` | Default value | `default:"8080"` |
| `required` | Must be set | `required:"true"` |
| `secret` | Mask in logs | `secret:"true"` |

### Supported Types

| Type | Example Value |
|:-----|:--------------|
| `string` | `"hello"` |
| `int`, `int64` | `42` |
| `float64` | `3.14` |
| `bool` | `true`, `false` |
| `time.Duration` | `30s`, `5m`, `1h` |
| `[]string` | `a,b,c` |
| Nested structs | See below |

### Nested Structs

```go
type Config struct {
    Server struct {
        Host string `default:"0.0.0.0"`
        Port int    `default:"8080"`
    }
    Database struct {
        URL      string `required:"true"`
        PoolSize int    `default:"10"`
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
cfg, _ := envconfig.Load[Config](
    envconfig.WithPrefix("MYAPP"),
)
// Port → MYAPP_PORT
// DatabaseURL → MYAPP_DATABASE_URL
```

### Multiple Sources

```go
cfg, _ := envconfig.Load[Config](
    envconfig.WithProvider(envconfig.Defaults[Config]()), // 1️⃣ Defaults
    envconfig.WithProvider(envconfig.File("config.json")), // 2️⃣ File
    envconfig.WithProvider(envconfig.Env()),               // 3️⃣ Environment
)
```

### Custom Validation

```go
cfg, err := envconfig.Load[Config](
    envconfig.WithValidator(func(cfg any) error {
        c := cfg.(*Config)
        if c.Port < 1024 {
            return errors.New("port must be >= 1024")
        }
        return nil
    }),
)
```

### Hot Reload

```go
loader := envconfig.NewLoader[Config](
    envconfig.WithProvider(envconfig.File("config.json")),
    envconfig.WithWatch("config.json", 5*time.Second),
    envconfig.WithOnReload(func() {
        log.Println("⚡ Config reloaded!")
    }),
)

cfg := loader.MustLoad()
loader.StartWatching()
defer loader.StopWatching()

// Get latest config anytime
current := loader.Get()
```

### Custom Provider

```go
type VaultProvider struct {
    Address string
}

func (p *VaultProvider) Values() (map[string]string, error) {
    // Fetch from Vault, AWS SSM, etc.
    return map[string]string{
        "JWT_SECRET": "secret-from-vault",
    }, nil
}

cfg, _ := envconfig.Load[Config](
    envconfig.WithProvider(&VaultProvider{Address: "vault:8200"}),
)
```

---

## 🖨️ Printing Config

```go
envconfig.Print(cfg)
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

> 🔐 Secrets are automatically masked based on field name or `secret:"true"` tag.

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
        Host         string        `default:"0.0.0.0"`
        Port         int           `default:"8080"`
        ReadTimeout  time.Duration `default:"5s"`
        WriteTimeout time.Duration `default:"10s"`
    }
    Database struct {
        URL         string `required:"true" secret:"true"`
        MaxConns    int    `default:"25"`
        MaxIdleTime time.Duration `default:"5m"`
    }
    Auth struct {
        JWTSecret     string        `required:"true" secret:"true"`
        TokenExpiry   time.Duration `default:"24h"`
        RefreshExpiry time.Duration `default:"168h"`
    }
    Features struct {
        Debug      bool     `default:"false"`
        CORSOrigins []string `default:"http://localhost:3000"`
    }
}

func main() {
    cfg := envconfig.MustLoad[Config]()
    envconfig.Print(cfg)
    
    // Use cfg.Server.Port, cfg.Database.URL, etc.
}
```

</details>

<details>
<summary><b>With Validation</b></summary>

```go
type Config struct {
    Port     int    `default:"8080"`
    LogLevel string `default:"info"`
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
    cfg, err := envconfig.Load[Config]()
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
    
    loader := envconfig.NewLoader[Config](
        envconfig.WithProvider(envconfig.Defaults[Config]()),
    )
    
    // Load environment-specific file
    switch env {
    case "production":
        loader = envconfig.NewLoader[Config](
            envconfig.WithProvider(envconfig.Defaults[Config]()),
            envconfig.WithProvider(envconfig.File("config.prod.json")),
            envconfig.WithProvider(envconfig.Env()),
        )
    case "staging":
        loader = envconfig.NewLoader[Config](
            envconfig.WithProvider(envconfig.Defaults[Config]()),
            envconfig.WithProvider(envconfig.File("config.staging.json")),
            envconfig.WithProvider(envconfig.Env()),
        )
    default:
        loader = envconfig.NewLoader[Config](
            envconfig.WithProvider(envconfig.Defaults[Config]()),
            envconfig.WithProvider(envconfig.File("config.local.json")),
            envconfig.WithProvider(envconfig.Env()),
        )
    }
    
    return loader.MustLoad()
}
```

</details>

---

## 📚 API Reference

### Load Functions

```go
cfg, err := envconfig.Load[T](opts...)    // Load with error
cfg := envconfig.MustLoad[T](opts...)      // Load or panic
```

### Options

```go
envconfig.WithPrefix(prefix)        // Env var prefix
envconfig.WithProvider(p)           // Add provider
envconfig.WithValidator(fn)         // Custom validator
envconfig.WithWatch(path, interval) // File watching
envconfig.WithOnReload(fn)          // Reload callback
envconfig.WithOutput(w)             // Print writer
```

### Providers

```go
envconfig.Defaults[T]()             // Struct tag defaults
envconfig.Env()                     // Environment variables
envconfig.File(path)                // JSON file
envconfig.Map(m)                    // String map
```

### Loader (Hot Reload)

```go
loader := envconfig.NewLoader[T](opts...)
loader.Load()          // Load config
loader.MustLoad()      // Load or panic
loader.Get()           // Get current config
loader.Version()       // Get version number
loader.StartWatching() // Start file watcher
loader.StopWatching()  // Stop file watcher
```

### Errors

```go
envconfig.ErrRequired        // Required field empty
envconfig.ErrValidation      // Validation failed
envconfig.ErrParse           // Parse error
envconfig.ErrUnsupportedType // Unsupported type
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

**Made with ❤️ by [Nicolas MMB](https://github.com/nicolasmmb)**

⭐ Star this repo if you find it useful!

</div>
