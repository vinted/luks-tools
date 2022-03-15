package keyfile

import (
	"os"
)

func CheckKeyfile(path string) bool {
	_, err := os.Stat(path + "/secret.key")
	return !os.IsNotExist(err)
}

func WriteKeyfile(path string, data []byte) error {
	err := os.WriteFile(path+"/secret.key", data, 0600)
	if err != nil {
		return err
	}
	return nil
}
