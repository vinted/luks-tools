module github.com/vinted/luks-tools

go 1.13

require (
	github.com/aws/aws-sdk-go v1.42.8
	github.com/gorilla/mux v1.8.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/sirupsen/logrus v1.8.1
	github.com/thanos-io/thanos v0.24.0
	github.com/vinted/certificator v0.1.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	gopkg.in/yaml.v2 v2.4.0
)

replace k8s.io/client-go => k8s.io/client-go v0.20.4
