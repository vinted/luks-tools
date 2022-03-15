package sink

import (
	"bufio"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/config"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/s3store"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

func authorizedKeys() map[string]bool {
	cfg := config.Config
	authorizedKeysBytes, err := ioutil.ReadFile(cfg.AuthorizedKeysFile)
	if err != nil {
		log.Error("Failed to load authorized_keys, err: ", err)
	}
	authorizedKeysMap := map[string]bool{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			log.Error(err)
		}
		authorizedKeysMap[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}
	return authorizedKeysMap
}

func sshAddHostKey(sshConfig *ssh.ServerConfig) error {
	cfg := config.Config
	privateBytes, err := ioutil.ReadFile(cfg.PrivateKeyFile)
	if err != nil {
		return err
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return err
	}
	sshConfig.AddHostKey(private)
	return nil
}

func sshConfig() (*ssh.ServerConfig, error) {
	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKeys()[string(pubKey.Marshal())] {
				return &ssh.Permissions{
					// Record the public key used for authentication.
					Extensions: map[string]string{
						"pubkey-fp": ssh.FingerprintSHA256(pubKey),
					},
				}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", c.User())
		},
		ServerVersion: "SSH-2.0-OpenSSH_8.1",
	}
	err := sshAddHostKey(sshConfig)
	if err != nil {
		log.Error("Failed to read private key: ", err)
		return nil, err
	}

	return sshConfig, nil
}

func StartSSHServer() error {
	cfg := config.Config
	sshConfig, err := sshConfig()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.SshBindAddr)
	if err != nil {
		log.Error("failed to listen for connection: ", err)
		return err
	}
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			return err
		}
		_, chans, reqs, err := ssh.NewServerConn(tcpConn, sshConfig)
		if err != nil {
			log.Error(err)
		}
		go ssh.DiscardRequests(reqs)
		go handleChannels(chans)
	}
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Error("Unable to handle channel: ", err)
	}
	go func(in <-chan *ssh.Request) {
		for req := range in {
			log.Info("Request type: ", req.Type)
			if req.Type == "exec" {
				payload := req.Payload
				switch {
				case strings.Contains(string(req.Payload), "scp "):
					err := scpSink(channel, payload)
					if err != nil {
						log.Error("Failed to run scpSink: ", err)
					}
				case strings.Contains(string(req.Payload), "dd "):
					err := sshSink(channel, payload)
					if err != nil {
						log.Error("Failed to run sshSink: ", err)
					}
				case strings.Contains(string(req.Payload), "df "):
					err := dummyDf(channel, payload)
					if err != nil {
						log.Error("Failed to run dummyDf: ", err)
					}
				default:
					err := dummyCommand(channel, payload)
					if err != nil {
						log.Error("Failed to run dummyCommand: ", err)
					}
				}
			}
			if req.Type == "shell" {
				err := dummyShell(channel)
				if err != nil {
					log.Error("Failed to run dummyShell: ", err)
				}
			}
		}
	}(requests)
}

func dummyShell(channel ssh.Channel) error {
	defer channel.Close()
	log.Info("Executing dummy shell")
	err := writeToChannel(channel, []byte("Shell NotImplemented\n"))
	if err != nil {
		return err
	}
	// send exit status of 0 via ssh channel
	err = exitSuccess(channel)
	if err != nil {
		return err
	}
	return nil
}

func dummyDf(channel ssh.Channel, payload []byte) error {
	// acts as "df -P" command. Sends dummy info about free space
	defer channel.Close()
	log.Info("Executing dummy df: ", string(payload))
	err := writeToChannel(channel, []byte("Filesystem              1024-blocks     Used Available Capacity Mounted on\n"))
	if err != nil {
		return err
	}
	err = writeToChannel(channel, []byte("/dev/sda   240000000 0 240000000       0% /\n"))
	if err != nil {
		return err
	}
	// send exit status of 0 via ssh channel
	err = exitSuccess(channel)
	if err != nil {
		return err
	}
	return nil
}

func dummyCommand(channel ssh.Channel, payload []byte) error {
	// dummy df. Sends reply to kdump.sh about free disk space
	defer channel.Close()
	err := writeToChannel(channel, []byte("NotImplemented\n"))
	if err != nil {
		return err
	}
	log.Info("Executing dummy command: ", string(payload))
	// send exit status of 0 via ssh channel
	err = exitSuccess(channel)
	if err != nil {
		return err
	}
	return nil
}

func sshSink(channel ssh.Channel, payload []byte) error {
	// reads stream from ssh
	defer channel.Close()
	log.Info("Executing ssh sink")
	log.Info("Sink command: ", string(payload))
	err := s3store.Upload(io.Reader(channel), payload, -1)
	if err != nil {
		return err
	}
	// send exit status of 0 via ssh channel
	err = exitSuccess(channel)
	if err != nil {
		return err
	}
	return nil
}

func scpSink(channel ssh.Channel, payload []byte) error {
	// reads stream as scp sink.
	defer channel.Close()
	log.Info("Executing scp sink")
	//first ACK
	ack := []byte{0}
	err := writeToChannel(channel, ack)
	if err != nil {
		return err
	}
	// read command
	buffer := make([]uint8, 1)
	_, err = channel.Read(buffer)
	if err != nil {
		log.Error("Failed to read from channel: ", err)
		return err
	}
	log.Info("Command: ", string(buffer[0]))
	// read file info
	bufferedReader := bufio.NewReader(channel)
	message, err := bufferedReader.ReadString('\n')
	if err != nil {
		log.Error("Failed to read from buffer: ", err)
		return err
	}
	log.Info("File info: ", message)
	fileInfo := strings.Split(message, " ")
	fileSize, err := strconv.ParseInt(fileInfo[1], 10, 64)
	if err != nil {
		log.Error("Failed to parse int: ", err)
		return err
	}
	log.Info("File size: ", fileSize)
	// second ACK
	err = writeToChannel(channel, ack)
	if err != nil {
		return err
	}
	// read file contents
	err = s3store.Upload(io.Reader(channel), payload, fileSize)
	if err != nil {
		return err
	}
	// third ack
	err = writeToChannel(channel, ack)
	if err != nil {
		return err
	}
	// send exit status of 0 via ssh channel
	err = exitSuccess(channel)
	if err != nil {
		return err
	}
	return nil
}

func exitSuccess(channel ssh.Channel) error {
	_, err := channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
	if err != nil {
		return err
	}
	return nil
}

func writeToChannel(channel ssh.Channel, message []byte) error {
	_, err := channel.Write(message)
	if err != nil {
		return err
	}
	return nil
}
