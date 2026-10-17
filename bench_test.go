package envx

import (
	"io"
	"testing"
	"time"
)

type benchSmallConfig struct {
	Port    int           `envx:"name=BENCH_PORT,default=8080"`
	Debug   bool          `envx:"name=BENCH_DEBUG,default=false"`
	Timeout time.Duration `envx:"name=BENCH_TIMEOUT,default=30s"`
}

type benchEnv string

const (
	benchEnvLocal      benchEnv = "local"
	benchEnvStaging    benchEnv = "staging"
	benchEnvProduction benchEnv = "production"
)

func (benchEnv) EnumValues() []string {
	return []string{
		string(benchEnvLocal),
		string(benchEnvStaging),
		string(benchEnvProduction),
	}
}

type benchComplexConfig struct {
	Name string   `envx:"name=NAME,default=svc"`
	Env  benchEnv `envx:"name=ENV,default=local"`
	Port int      `envx:"name=PORT,default=8080"`

	Database *struct {
		URL      string `envx:"name=URL,required=true"`
		PoolSize *int   `envx:"name=POOL_SIZE,default=10"`
	}

	Features struct {
		Debug          *bool     `envx:"name=DEBUG,default=false"`
		AllowedOrigins *[]string `envx:"name=ALLOWED_ORIGINS,default=http://localhost:3000"`
	}

	Timeout *time.Duration `envx:"name=TIMEOUT,default=30s"`
}

var (
	benchResultSmall   *benchSmallConfig
	benchResultComplex *benchComplexConfig
)

func BenchmarkLoad_DefaultStackSmall(b *testing.B) {
	b.Setenv("BENCH_PORT", "9090")
	b.Setenv("BENCH_DEBUG", "true")
	b.Setenv("BENCH_TIMEOUT", "45s")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg, err := Load[benchSmallConfig]()
		if err != nil {
			b.Fatalf("Load: %v", err)
		}
		benchResultSmall = cfg
	}
}

func BenchmarkLoad_ComplexPrefixPointers(b *testing.B) {
	b.Setenv("APPBENCH_NAME", "bench-api")
	b.Setenv("APPBENCH_ENV", "staging")
	b.Setenv("APPBENCH_PORT", "8181")
	b.Setenv("APPBENCH_DATABASE_URL", "postgres://localhost/bench")
	b.Setenv("APPBENCH_DEBUG", "true")
	b.Setenv("APPBENCH_ALLOWED_ORIGINS", "https://a.example.com,https://b.example.com")
	b.Setenv("APPBENCH_TIMEOUT", "90s")

	opts := []Option{
		WithPrefix("APPBENCH"),
		WithPrecedence(PrecedenceEnvWins),
		WithProvider(Map(map[string]string{
			"DATABASE_POOL_SIZE": "25",
		})),
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cfg, err := Load[benchComplexConfig](opts...)
		if err != nil {
			b.Fatalf("Load: %v", err)
		}
		benchResultComplex = cfg
	}
}

func BenchmarkParseAndValidate_ComplexMap(b *testing.B) {
	values := map[string]any{
		"APPBENCH_NAME":                   "bench-api",
		"APPBENCH_ENV":                    "production",
		"APPBENCH_PORT":                   "8082",
		"APPBENCH_DATABASE_URL":           "postgres://localhost/bench",
		"APPBENCH_DATABASE_POOL_SIZE":     "20",
		"APPBENCH_DEBUG":                  "true",
		"APPBENCH_ALLOWED_ORIGINS":        "https://a.example.com,https://b.example.com",
		"APPBENCH_TIMEOUT":                "75s",
		"APPBENCH_UNUSED_EXTRA_PARAMETER": "noop",
	}
	logger := newWriterLogger(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var cfg benchComplexConfig
		if err := parse(&cfg, values, "APPBENCH", logger); err != nil {
			b.Fatalf("parse: %v", err)
		}
		if err := validateRequiredAgainstValues(&cfg, values, "APPBENCH"); err != nil {
			b.Fatalf("validateRequired: %v", err)
		}
		benchResultComplex = &cfg
	}
}
