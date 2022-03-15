package dmtune

import (
	"testing"
)

func TestDriveConfigured(t *testing.T) {
	table := "sdd: 0 13 crypt aes-xts-plain64 0 0 8:48 4096 3 allow_discards no_read_workqueue no_write_workqueue"
	if IsDeviceConfigured(table) != true {
		t.Fatal("IsDeviceConfigured() failed")
	}
}

func TestDriveEnabled(t *testing.T) {
	drives := []string{"sda", "sdb"}
	if driveEnabled(drives, "sdb") != true {
		t.Fatal("driveEnabled() failed")
	}
}
