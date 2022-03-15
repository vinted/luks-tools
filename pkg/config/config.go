package config

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Cfg struct {
	ManageNetwork bool   `yaml:"manage_network"`
	KeystorePath  string `yaml:"keystore_path"`
	Plugin        string `yaml:"plugin"`
}

func ParseConfig() (Cfg, error) {
	var cfg Cfg

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
