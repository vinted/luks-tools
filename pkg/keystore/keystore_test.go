package keystore

import (
	"testing"
)

func TestIsTmpfs(t *testing.T) {
	if checkDirectoryIsTmpfs("/dev") != true {
		t.Fatal("checkDirectoryIsTmpfs(\"/dev\") failed")
	}
}

func TestDirectoryExists(t *testing.T) {
	if checkDirectoryExists("/root") != true {
		t.Fatal("checkDirectoryExists(\"/root\") failed")
	}
}
