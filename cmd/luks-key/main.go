package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/config"
	"github.com/vinted/luks-tools/pkg/keyfile"
	"github.com/vinted/luks-tools/pkg/keystore"
	"github.com/vinted/luks-tools/pkg/network"
	"os"
	"plugin"
)

func main() {
	cfg, err := config.ParseConfig()

	if err != nil {
		os.Exit(0)
	}

	if keyfile.CheckKeyfile(cfg.KeystorePath) {
		log.Info("Keyfile exists. Exiting.")
		os.Exit(0)
	}

	log.Info("Using plugin: ", cfg.Plugin)

	if cfg.ManageNetwork {
		network.Up()
	}

	var plug *plugin.Plugin
	plug, err = plugin.Open(cfg.Plugin)
	if err != nil {
		log.Error("Failed opening plugin ", cfg.Plugin, ". Got error: ", err)
		finish(cfg.ManageNetwork)
	}

	symGetKey, err := plug.Lookup("GetKey")
	if err != nil {
		log.Error("Failed looking up symbol GetKey. Got error: ", err)
		finish(cfg.ManageNetwork)
	}

	getKey := symGetKey.(func() (string, error))

	var key string
	key, err = getKey()

	if err != nil {
		log.Error("Failed to open file. Got error: ", err)
		finish(cfg.ManageNetwork)
	}

	keystore.MountKeystore(cfg)
	err = keyfile.WriteKeyfile(cfg.KeystorePath, []byte(key))
	if err != nil {
		log.Error("Failed to write keyfile. Got error:  ", err)
		finish(cfg.ManageNetwork)
	}

	finish(cfg.ManageNetwork)
}

func finish(manageNetwork bool) {
	if manageNetwork {
		network.Down()
	}
	os.Exit(0)
}
