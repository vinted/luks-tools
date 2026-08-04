package network

import (
	"bytes"
	"net"
	"os/exec"
	"time"

	"github.com/vinted/luks-tools/pkg/config"

	log "github.com/sirupsen/logrus"
)

func Down() {
	log.Info("Bringing network down")
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Error(err)
	}
	err = exec.Command("pkill", "-9", "dhclient").Run()
	if err != nil {
		log.Error(err)
	}
	for _, iface := range interfaces {
		// Skip non-ethernet devices
		if len(iface.HardwareAddr) != 6 {
			continue
		}
		log.Info("Removing configuration from ", iface.Name)
		err := exec.Command("ip", "addr", "flush", "dev", iface.Name).Run()
		if err != nil {
			log.Error(err)
		}
		err = exec.Command("ip", "route", "flush", "dev", iface.Name).Run()
		if err != nil {
			log.Error(err)
		}
		err = exec.Command("ip", "link", "set", iface.Name, "down").Run()
		if err != nil {
			log.Error(err)
		}
	}
}

func Up(cfg config.Cfg) {
	log.Info("Bringing network up")
	startDHCPClient(cfg)
}

func startDHCPClient(cfg config.Cfg) {
	args := []string{
		"-sf", "/usr/sbin/luks-dhcp.sh",
	}
	// Wait for network drivers to load
	log.Info("Waiting for interface drivers to load")
	d, err := time.ParseDuration(cfg.WaitForNetwork)
	if err != nil {
		log.Error(err)
	}
	time.Sleep(d)

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Error(err)
	}
	for _, iface := range interfaces {
		// Skip non-ethernet devices
		if len(iface.HardwareAddr) != 6 {
			continue
		}
		args = append(args, iface.Name)
		err := exec.Command("ip", "link", "set", iface.Name, "up").Run()
		if err != nil {
			log.Error(err)
		}
	}

	cmd := exec.Command("dhclient", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			log.Error(stderr.String())
		} else {
			log.Info(stdout.String())
		}
	}

	// Sleep for 5 seconds for network to be really up
	log.Info("Sleeping 5s")
	time.Sleep(5 * time.Second)
}
