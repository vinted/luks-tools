package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/vinted/certificator/pkg/vault"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type vaultPluginCfg struct {
	PluginConfigVaultAddress     string `yaml:"plugin_config_vault_address"`
	PluginConfigVaultStoragePath string `yaml:"plugin_config_vault_storage_path"`
	PluginConfigVaultRoleId      string `yaml:"plugin_config_vault_role_id"`
	PluginConfigVaultSecretId    string `yaml:"plugin_config_vault_secret_id"`
	PluginConfigVaultSkipVerify  bool   `yaml:"plugin_config_vault_skip_verify"`
	PluginConfigServerId         string `yaml:"plugin_config_server_id"`
}

func parsePluginConfig() (vaultPluginCfg, error) {
	var cfg vaultPluginCfg
	config_path := "/etc/luks-tools/config.yml"

	if os.Getenv("LUKS_TOOLS_CFG_PATH") != "" {
		config_path = os.Getenv("LUKS_TOOLS_CFG_PATH")
	}

	f, err := ioutil.ReadFile(config_path)

	if err != nil {
		log.Error("Error reading config file", err)
		return cfg, err
	}

	err = yaml.Unmarshal(f, &cfg)
	if err != nil {
		log.Error("Error parsing config yaml", err)
		return cfg, err
	}
	return cfg, nil
}

func GetKey() (string, error) {
	cfg, err := parsePluginConfig()
	if err != nil {
		return "", err
	}

	keyStoragePath := cfg.PluginConfigVaultStoragePath
	logger := log.New()

	if cfg.PluginConfigVaultSkipVerify {
		os.Setenv("VAULT_SKIP_VERIFY", "true")
	}

	if os.Getenv("VAULT_ADDR") == "" {
		os.Setenv("VAULT_ADDR", cfg.PluginConfigVaultAddress)
	} else {
		log.Info("Got Vault address via environment variable")
	}

	vaultClient, err := vault.NewVaultClient(cfg.PluginConfigVaultRoleId,
		cfg.PluginConfigVaultSecretId, "prod", keyStoragePath, logger)
	if err != nil {
		return "", err
	}
	var key map[string]interface{}
	key, err = vaultClient.KVRead(cfg.PluginConfigServerId)

	if err != nil {
		return "", err
	}

	return key["key"].(string), nil
}
