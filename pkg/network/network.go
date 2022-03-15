package network

import (
	log "github.com/sirupsen/logrus"
	"net"
	"os/exec"
	"time"
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

func Up() {
	log.Info("Bringing network up")
	startDHCPClient()
}

func startDHCPClient() {
	// Wait for network drivers to load
	log.Info("Waiting for interface drivers to load")
	time.Sleep(3 * time.Second)

	interfaces, err := net.Interfaces()
	if err != nil {
		log.Error(err)
	}
	for _, iface := range interfaces {
		err := exec.Command("ip", "link", "set", iface.Name, "up").Run()
		if err != nil {
			log.Error(err)
		}
	}
	err = exec.Command("dhclient", "-sf", "/usr/sbin/luks-dhcp.sh").Run()
	if err != nil {
		log.Error("DHCP client failed. Got error: ", err)
	}

	// Sleep for 5 seconds for network to be really up
	time.Sleep(5 * time.Second)
}
