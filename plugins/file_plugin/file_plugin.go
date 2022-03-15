package main

/*
Sample plugin. Reads file content from file defined in
config file as `plugin_config_file_path` and returns result as string.
*/

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type filePluginCfg struct {
	PluginConfigFilePath string `yaml:"plugin_config_file_path"`
}

func parsePluginConfig() (filePluginCfg, error) {
	var cfg filePluginCfg

	f, err := ioutil.ReadFile("/etc/luks-tools/config.yml")
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
	var content []byte

	cfg, err := parsePluginConfig()
	if err != nil {
		return "", err
	}

	content, err = ioutil.ReadFile(cfg.PluginConfigFilePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}
