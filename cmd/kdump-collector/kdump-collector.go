package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/config"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/sink"
)

func main() {
	config.Config = config.ParseConfig()
	log.Info("kdump-collector service started")
	err := sink.StartSSHServer()
	if err != nil {
		log.Fatal("Failed to start SSH server: ", err)
	}
}
