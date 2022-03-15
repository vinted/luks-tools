package sink

import (
	"bytes"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/config"
	"io"
	"os"
	"testing"
)

// mock ssh.Channel interface
type channelMock struct{}

var channelRequest []byte
var channelData string

func (c channelMock) Close() error {
	return nil
}

func (c channelMock) CloseWrite() error {
	return nil
}

func (c channelMock) Read(data []byte) (int, error) {
	return 1, nil
}

func (c channelMock) Write(data []byte) (int, error) {
	channelData = string(data)
	return 1, nil
}

func (c channelMock) SendRequest(name string, wantReply bool, payload []byte) (bool, error) {
	channelRequest = payload
	return true, nil
}

func (c channelMock) Stderr() io.ReadWriter {
	return nil
}

func setEnv() {
	os.Setenv("KDUMP_S3_ENDPOINT", "http://localhost")
	os.Setenv("KDUMP_S3_ACCESS_KEY_ID", "keyId")
	os.Setenv("KDUMP_S3_SECRET_KEY", "secretKey")
	config.Config = config.ParseConfig()
}

func TestDummyShell(t *testing.T) {
	setEnv()
	var channel channelMock

	err := dummyShell(channel)
	if err != nil {
		t.Fatal("Got error: ", err)
	}
	if channelData != "Shell NotImplemented\n" {
		t.Fatal("Wrong reply received: ", channelData)
	}
	if !bytes.Equal(channelRequest, []byte{0, 0, 0, 0}) {
		t.Fatal("Wrong request received: ", channelRequest)
	}
	os.Clearenv()
}

func TestDummyDf(t *testing.T) {
	setEnv()
	var channel channelMock

	err := dummyDf(channel, []byte("df -P /var/crash"))
	if err != nil {
		t.Fatal("Got error: ", err)
	}
	if channelData != "/dev/sda   240000000 0 240000000       0% /\n" {
		t.Fatal("Wrong reply received: ", channelData)
	}
	if !bytes.Equal(channelRequest, []byte{0, 0, 0, 0}) {
		t.Fatal("Wrong request received: ", channelRequest)
	}
	os.Clearenv()
}
