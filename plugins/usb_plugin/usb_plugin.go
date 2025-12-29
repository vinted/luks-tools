package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func GetKey() (string, error) {
	var content []byte

	time.Sleep(10 * time.Second)

	err := os.Mkdir("/tmp/luks-tools", 0700)
	if err != nil {
		log.Error("Error creating directory: ", err)
	}

	entries, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		log.Error("Error getting device list: ", err)
	}

	for _, partition := range entries {
		if strings.Contains(partition.Name(), "usb") && strings.Contains(partition.Name(), "part") {
			fp := fmt.Sprintf("/dev/disk/by-id/%s", partition.Name())
			log.Info("Mounting", fp)
			cmd := exec.Command("mount", fp, "/tmp/luks-tools")
			_, err := cmd.Output()

			if err != nil {
				log.Error("Error executing command: ", err)
			}

			if _, err := os.Stat("/tmp/luks-tools/secret.key"); err == nil {
				content, err = os.ReadFile("/tmp/luks-tools/secret.key")
				if err != nil {
					log.Error("Error reading file: ", err)
				} else {
					break
				}
			}
			err = unmountVolume()
			if err != nil {
				log.Error("Error unmounting volume: ", err)
			}
		}

	}

	err = unmountVolume()
	if err != nil {
		log.Error("Error unmounting volume: ", err)
	}

	err = os.Remove("/tmp/luks-tools")
	if err != nil {
		log.Error("Error removing directory: ", err)
	}
	return string(content), err
}

func unmountVolume() error {
	cmd := exec.Command("umount", "/tmp/luks-tools")
	_, err := cmd.Output()

	return err
}
