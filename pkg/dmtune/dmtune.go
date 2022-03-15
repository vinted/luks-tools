package dmtune

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

type Cfg struct {
	DrivesEnabled []string `yaml:"dm_tune_drives"`
}

func DriveConfigurationEnabled(dmDevice string) bool {
	cfg, err := parseConfig()
	if err != nil {
		log.Error(err)
		os.Exit(0)
	}
	return driveEnabled(cfg.DrivesEnabled, dmDevice)
}

func driveEnabled(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func parseConfig() (Cfg, error) {
	var cfg Cfg

	f, err := ioutil.ReadFile("/etc/luks-tools/config.yml")
	if err != nil {
		return cfg, err
	}
	err = yaml.Unmarshal(f, &cfg)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func ConfigureDm(dmDevice string, table string) error {
	log.Info("Configuring device: ", dmDevice)
	var pipe = table + " 3 allow_discards no_read_workqueue no_write_workqueue"
	cmd := exec.Command("dmsetup", "reload", dmDevice)
	cmd.Stdin = strings.NewReader(pipe)
	err := cmd.Run()
	if err != nil {
		return err
	}
	err = exec.Command("dmsetup", "suspend", dmDevice).Run()
	if err != nil {
		return err
	}
	err = exec.Command("dmsetup", "resume", dmDevice).Run()
	if err != nil {
		return err
	}
	return nil
}

func DMTable(dmDevice string) (string, error) {
	table, err := exec.Command("dmsetup", "table", dmDevice, "--showkeys").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(table), "\n"), nil
}

func IsDeviceConfigured(table string) bool {
	switch {
	case strings.Contains(table, "allow_discards"):
		return true
	case strings.Contains(table, "no_read_workqueue"):
		return true
	case strings.Contains(table, "no_write_workqueue"):
		return true
	default:
		return false
	}
}
