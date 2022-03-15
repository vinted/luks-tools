package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/dmtune"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		dmDevice := os.Args[1]
		log.Info("Configuring dm device: " + dmDevice)
		if !dmtune.DriveConfigurationEnabled(dmDevice) {
			log.Info("Configuration for device ", dmDevice, " is disabled")
			os.Exit(0)
		}
		table, err := dmtune.DMTable(dmDevice)
		if err != nil {
			log.Error(err)
			os.Exit(0)
		}
		log.Info("dm table: " + table)
		if dmtune.IsDeviceConfigured(table) {
			log.Info("Device is already configured")
			os.Exit(0)
		}
		err = dmtune.ConfigureDm(dmDevice, table)
		if err != nil {
			log.Error(err)
			os.Exit(0)
		}
	} else {
		log.Info("No dm device specified")
	}
}
