package config

import (
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

var Config *Cfg

type Cfg struct {
	StripFromBucketName string `envconfig:"KDUMP_STRIP_FROM_NAME" default:"/var/crash/"`
	AuthorizedKeysFile  string `envconfig:"KDUMP_SSH_AUTHORIZED_KEYS_FILE" default:"authorized_keys"`
	PrivateKeyFile      string `envconfig:"KDUMP_SSH_PRIVATE_KEY_FILE" default:"private.key"`
	SshBindAddr         string `envconfig:"KDUMP_SSH_BIND_ADDRESS" default:"0.0.0.0:22"`
	S3ChunkSize         int64  `envconfig:"KDUMP_S3_CHUNK_SIZE" default:"536870912"`
	ReadBuffSize        int    `envconfig:"KDUMP_READ_BUFF_SIZE" default:"1024"`
	S3AccessKeyID       string `envconfig:"KDUMP_S3_ACCESS_KEY_ID" required:"true"`
	S3SecretKey         string `envconfig:"KDUMP_S3_SECRET_KEY" required:"true"`
	S3Endpoint          string `envconfig:"KDUMP_S3_ENDPOINT" required:"true"`
	S3Region            string `envconfig:"KDUMP_S3_EREGION" default:"us-east-1"`
	S3ForcePathStyle    bool   `envconfig:"KDUMP_S3_FORCE_PATH_STYLE" default:"true"`
}

func ParseConfig() *Cfg {
	var cfg Cfg
	err := envconfig.Process("", &cfg)
	if err != nil {
		log.Fatal("Error retrieving configuration: ", err.Error())
	}
	return &cfg
}
