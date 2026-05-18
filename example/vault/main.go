//go:build vault

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/vault/api"
	"github.com/nicolasmmb/envx"
)

type vaultProvider struct {
	client *api.Client
	path   string
}

func (p *vaultProvider) Values() (map[string]any, error) {
	secret, err := p.client.KVv2("kv").Get(context.Background(), p.path)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	for k, v := range secret.Data {
		values[k] = v
	}
	return values, nil
}

type Config struct {
	DatabaseURL string `envx:"name=DATABASE_URL,required=true"`
	AppEnv      string `envx:"name=APP_ENV,enum=\"dev,staging,prod\""`
	LegacyToken string `envx:"name=LEGACY_TOKEN,deprecated=true"`
}

func main() {
	cfg, err := loadConfigFromVault()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	envx.Print(cfg)
	fmt.Printf("DatabaseURL=%s AppEnv=%s\n", cfg.DatabaseURL, cfg.AppEnv)
}

func loadConfigFromVault() (*Config, error) {
	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		return nil, fmt.Errorf("VAULT_ADDR not set")
	}

	cfg := api.DefaultConfig()
	cfg.Address = vaultAddr
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		client.SetToken(token)
	}

	return envx.Load[Config](
		envx.WithProvider(&vaultProvider{client: client, path: "app/config"}),
	)
}
