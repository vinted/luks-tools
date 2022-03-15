package keyfile

import (
	"testing"
)

func TestKeyfileExists(t *testing.T) {
	if CheckKeyfile("/var/test/keyfile/") != false {
		t.Fatal("CheckKeyfile(\"/var/test/keyfile/\") failed")
	}
}
