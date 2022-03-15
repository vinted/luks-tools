package keystore

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/config"
	"os"
	"syscall"
)

func MountKeystore(cfg config.Cfg) {
	if !checkDirectoryExists(cfg.KeystorePath) {
		err := createDirectory(cfg.KeystorePath)
		if err != nil {
			log.Error("Failed creating directory ", cfg.KeystorePath, " ", err)
			os.Exit(0)
		}
	}

	if !checkDirectoryIsTmpfs(cfg.KeystorePath) {
		err := mountDirectoryAsTmpfs(cfg.KeystorePath)
		if err != nil {
			log.Error("Failed mounting directory as tmpfs. Got error: ", err)
			os.Exit(0)
		}
	} else {
		return
	}
	// Do a final check. At this point keystore MUST be on a tmpfs.
	// If its still not so, - exit with error.
	if !checkDirectoryIsTmpfs(cfg.KeystorePath) {
		log.Error("Filesystem is still not on tmpfs")
		os.Exit(0)
	}
}

func checkDirectoryExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Info("Keystore ", path, " is missing")
		return false
	}
	return true
}

func checkDirectoryIsTmpfs(path string) bool {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		log.Error("Failed checking directory filesystem type. Got error: ", err)
		os.Exit(0)
	}

	// https://man7.org/linux/man-pages/man2/statfs.2.html
	log.Info("Filesystem on ", path, " has magic number of ", fmt.Sprintf("%0x", stat.Type), "h")
	if stat.Type == 16914836 {
		log.Info("Filesystem is tmpfs")
		return true
	}
	log.Info("Filesystem is not tmpfs")
	return false
}

func createDirectory(path string) error {
	err := os.Mkdir(path, 0700)
	if err != nil {
		return err
	}
	return nil
}

func mountDirectoryAsTmpfs(path string) error {
	var mountFlags uintptr

	// https://man7.org/linux/man-pages/man2/mount.2.html
	mountFlags = syscall.MS_SILENT | syscall.MS_NOSUID
	mountFlags |= syscall.MS_NODEV | syscall.MS_NOEXEC

	log.Info("Mounting tmpfs on ", path)
	err := syscall.Mount("tmpfs", path, "tmpfs", mountFlags, "mode=700,size=4096")

	if err != nil {
		return err
	}
	return nil
}
