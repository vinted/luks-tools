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
	"time"
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
			// rest is not set on error, so nothing more can be parsed
			log.Error("Failed to parse authorized_keys: ", err)
			break
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
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				log.Error("Failed to accept connection: ", err)
				continue
			}
			return err
		}
		// handshake runs in its own goroutine, so a slow or stalled client
		// does not block other connections from being accepted.
		go handleConn(tcpConn, sshConfig, cfg.SshHandshakeTimeout)
	}
}

func handleConn(tcpConn net.Conn, sshConfig *ssh.ServerConfig, handshakeTimeout time.Duration) {
	// bound the handshake, otherwise a client that never completes it holds
	// the connection forever.
	err := tcpConn.SetDeadline(time.Now().Add(handshakeTimeout))
	if err != nil {
		log.Error("Failed to set handshake deadline: ", err)
		tcpConn.Close()
		return
	}
	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, sshConfig)
	if err != nil {
		// do not log EOF errors, as they most probably are caused by portscan
		// or heartbeats.
		if err != io.EOF {
			log.Error("SSH handshake with ", tcpConn.RemoteAddr(), " failed: ", err)
		}
		tcpConn.Close()
		return
	}
	// handshake done, dump transfer may take a long time
	err = tcpConn.SetDeadline(time.Time{})
	if err != nil {
		log.Error("Failed to clear handshake deadline: ", err)
		sshConn.Close()
		return
	}
	log.Info("SSH connection from ", sshConn.RemoteAddr(), " (", string(sshConn.ClientVersion()), ")")
	go ssh.DiscardRequests(reqs)
	handleChannels(chans)
}

func handleChannels(chans <-chan ssh.NewChannel) {
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			err := newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			if err != nil {
				log.Error("Failed to reject channel: ", err)
			}
			continue
		}
		go handleChannel(newChannel)
	}
}

func handleChannel(newChannel ssh.NewChannel) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Error("Unable to handle channel: ", err)
		return
	}
	// requests must be drained and replied to for the whole channel lifetime.
	// Leaving them unserviced while a dump is streamed stalls the connection
	// once the request queue fills up (e.g. with keepalive@openssh.com).
	started := false
	for req := range requests {
		log.Info("Request type: ", req.Type)
		if started || (req.Type != "exec" && req.Type != "shell") {
			replyRequest(req, false)
			continue
		}
		started = true
		replyRequest(req, true)
		if req.Type == "shell" {
			go runCommand(channel, "dummyShell", func() error { return dummyShell(channel) })
			continue
		}
		payload := req.Payload
		switch {
		case strings.Contains(string(payload), "scp "):
			go runCommand(channel, "scpSink", func() error { return scpSink(channel, payload) })
		case strings.Contains(string(payload), "dd "):
			go runCommand(channel, "sshSink", func() error { return sshSink(channel, payload) })
		case strings.Contains(string(payload), "df "):
			go runCommand(channel, "dummyDf", func() error { return dummyDf(channel, payload) })
		default:
			go runCommand(channel, "dummyCommand", func() error { return dummyCommand(channel, payload) })
		}
	}
}

func runCommand(channel ssh.Channel, name string, command func() error) {
	err := command()
	if err != nil {
		log.Error("Failed to run ", name, ": ", err)
		// let the client know the command failed instead of silently closing
		er := exitFailure(channel)
		if er != nil {
			log.Error("Failed to send exit status: ", er)
		}
	}
	channel.Close()
}

func replyRequest(req *ssh.Request, ok bool) {
	if !req.WantReply {
		return
	}
	err := req.Reply(ok, nil)
	if err != nil {
		log.Error("Failed to reply to ", req.Type, " request: ", err)
	}
}

func dummyShell(channel ssh.Channel) error {
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
	log.Info("Executing scp sink")
	//first ACK
	ack := []byte{0}
	err := writeToChannel(channel, ack)
	if err != nil {
		return err
	}
	// all reads must go through the same buffered reader: bufio may already
	// hold the beginning of the file contents after reading the header line.
	bufferedReader := bufio.NewReader(channel)
	// read command
	command, err := bufferedReader.ReadByte()
	if err != nil {
		log.Error("Failed to read from channel: ", err)
		return err
	}
	log.Info("Command: ", string(command))
	// read file info
	message, err := bufferedReader.ReadString('\n')
	if err != nil {
		log.Error("Failed to read from buffer: ", err)
		return err
	}
	log.Info("File info: ", message)
	fileInfo := strings.Split(message, " ")
	if len(fileInfo) < 3 {
		return fmt.Errorf("malformed scp file info: %q", message)
	}
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
	err = s3store.Upload(bufferedReader, payload, fileSize)
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

func exitFailure(channel ssh.Channel) error {
	_, err := channel.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
	return err
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
