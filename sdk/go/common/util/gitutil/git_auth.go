package gitutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type AuthMethod interface {
	transport.AuthMethod

	// For testing the SSH Agent, we want to be able to run the client config synchronously.
	publicKeys() ([]ssh.Signer, error)
}

// sshUserSettings allows us to inject mock SSH config.
type sshUserSettings interface {
	GetAllStrict(alias, key string) ([]string, error)
}

var _ sshUserSettings = (*ssh_config.UserSettings)(nil)

type sshAgentBroker interface {
	Available() bool
	New() (sshAgentImpl, error)
}

type sshAgentImpl interface {
	io.Closer

	Signers() ([]ssh.Signer, error)
}

type defaultSSHAgentBroker struct{}

var _ sshAgentBroker = (*defaultSSHAgentBroker)(nil)

// Available implements sshAgentBroker.
func (*defaultSSHAgentBroker) Available() bool {
	return sshagent.Available()
}

// New implements sshAgentBroker.
func (*defaultSSHAgentBroker) New() (sshAgentImpl, error) {
	agent, conn, err := sshagent.New()
	if err != nil {
		return nil, err
	}
	return &defaultSSHAgent{agent: agent, Closer: conn}, nil
}

type defaultSSHAgent struct {
	io.Closer

	agent agent.Agent
}

// Signers implements sshAgentImpl.
func (d *defaultSSHAgent) Signers() ([]ssh.Signer, error) {
	return d.agent.Signers()
}

var _ sshAgentImpl = (*defaultSSHAgent)(nil)

// DefaultSSHAuth is an implementation of gitssh.AuthMethod that provides a default set of SSH
// authentication methods, using a set of provided public keys and an ambient SSH agent if
// available.
type DefaultSSHAuth struct {
	user string

	signers        []ssh.Signer
	sshAgentBroker sshAgentBroker

	gitssh.HostKeyCallbackHelper
}

var _ AuthMethod = (*DefaultSSHAuth)(nil)

func NewDefaultSSHAuth(user string) DefaultSSHAuth {
	return DefaultSSHAuth{user: user}
}

func (s *DefaultSSHAuth) AddIdentityFiles(host string, settings sshUserSettings, passphrase string) error {
	path := identityFiles(settings, host)

	for _, key := range path {
		signer, err := parseKey(keyConfig{path: key, passphrase: passphrase})
		if err != nil {
			return err
		}
		if signer != nil {
			logging.V(9).Infof("[gitutils] Adding SSH identity file %q", key)
			s.signers = append(s.signers, signer)
		}
	}

	return nil
}

func (s *DefaultSSHAuth) AddKeyFiles(files ...string) error {
	for _, file := range files {
		signer, err := parseKey(keyConfig{path: file, failOnNotExists: true})
		if err != nil {
			return err
		}
		if signer != nil {
			logging.V(9).Infof("[gitutils] Adding SSH public key file %q", file)
			s.signers = append(s.signers, signer)
		}
	}
	return nil
}

func (s *DefaultSSHAuth) AddKeyFilesWithPassphrase(files []string, passphrase string) error {
	for _, file := range files {
		signer, err := parseKey(keyConfig{
			path:             file,
			passphrase:       passphrase,
			failOnNotExists:  true,
			failOnPassphrase: passphrase != "",
		})
		if err != nil {
			return err
		}
		if signer != nil {
			logging.V(9).Infof("[gitutils] Adding SSH public key file with passphrase %q", file)
			s.signers = append(s.signers, signer)
		}
	}
	return nil
}

func (s *DefaultSSHAuth) AddKeyPem(pemData ...[]byte) error {
	for _, data := range pemData {
		signer, err := parseKey(keyConfig{data: data})
		if err != nil {
			return err
		}
		if signer != nil {
			logging.V(9).Infof("[gitutils] Adding private key PEM")
			s.signers = append(s.signers, signer)
		}
	}
	return nil
}

func (s *DefaultSSHAuth) AddKeyPemWithPassphrase(pemData [][]byte, passphrase string) error {
	for _, data := range pemData {
		signer, err := parseKey(keyConfig{
			data:             data,
			passphrase:       passphrase,
			failOnPassphrase: true,
		})
		if err != nil {
			return err
		}
		if signer != nil {
			s.signers = append(s.signers, signer)
		}
	}
	return nil
}

func (s *DefaultSSHAuth) AddSSHAgentBroker(broker sshAgentBroker) error {
	if s.sshAgentBroker != nil {
		return errors.New("SSH agent already set")
	}

	s.sshAgentBroker = broker
	return nil
}

type keyConfig struct {
	path             string
	data             []byte
	passphrase       string
	failOnNotExists  bool
	failOnPassphrase bool
}

// SSH on a modern system (circa 2024, OpenSSH_8.9p1) will try:
//
// - ~/.ssh/id_rsa
// - ~/.ssh/id_ecdsa
// - ~/.ssh/id_ecdsa_sk
// - ~/.ssh/id_ed25519
// - ~/.ssh/id_ed25519_sk
// - ~/.ssh/id_xmss
// - ~/.ssh/id_dsa
//
// Of these:
// - id_dsa and id_xmss are not supported in Go
// - id_ecdsa_sk and id_ed25519_sk require interactive use with FIDO2
//
// Producing this list of default identity files:
var DefaultIdentityFiles = []string{
	"~/.ssh/id_rsa",
	"~/.ssh/id_ecdsa",
	"~/.ssh/id_ed25519",
}

// ClientConfig implements ssh.AuthMethod.
func (s *DefaultSSHAuth) ClientConfig() (*ssh.ClientConfig, error) {
	// Use known hosts alongside our signers, via SetHostKeyCallback
	return s.SetHostKeyCallback(&ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(s.publicKeys),
		},
	})
}

// publicKeys implements AuthMethod.
func (s *DefaultSSHAuth) publicKeys() ([]ssh.Signer, error) {
	signers := append([]ssh.Signer{}, s.signers...)

	if s.sshAgentBroker != nil && s.sshAgentBroker.Available() {
		agent, err := s.sshAgentBroker.New()
		if err != nil {
			logging.V(3).Infof("[gitutils] Skipping SSH agent, unable to connect to agent: %v", err)
		} else {
			defer func() {
				err := agent.Close()
				if err != nil {
					logging.V(3).Infof("[gitutils] Failed to close SSH agent: %v", err)
				}
			}()
			agentSigners, err := agent.Signers()
			if err != nil {
				logging.V(3).Infof("[gitutils] Skipping SSH agent, unable to get agent signers: %v", err)
			} else {
				signers = append(signers, agentSigners...)
			}
		}
	} else {
		logging.V(3).Infof("[gitutils] SSH agent not available, using local public keys only")
	}

	return signers, nil
}

// Name implements ssh.AuthMethod.
func (s *DefaultSSHAuth) Name() string {
	return "DefaultSSHAuth"
}

// String implements ssh.AuthMethod.
func (s *DefaultSSHAuth) String() string {
	return "DefaultSSHAuth"
}

func identityFiles(sshUserSettings sshUserSettings, host string) []string {
	identityFiles, err := sshUserSettings.GetAllStrict(host, "IdentityFile")
	if err != nil {
		logging.V(3).Infof("[gitutils] Error reading ssh config for host %q: %v", host, err)
	}
	if len(identityFiles) == 0 {
		logging.V(3).Infof("[gitutils] No IdentityFile found in ssh config for host %q", host)
	}
	if len(identityFiles) == 1 && identityFiles[0] == "~/.ssh/identity" {
		identityFiles = append(identityFiles, DefaultIdentityFiles...)
	}

	keys := make([]string, 0, len(identityFiles))
	for _, file := range identityFiles {
		path, err := expandHomeDir(file)
		if err != nil {
			logging.V(3).Infof("[gitutils] Skipping invalid IdentityFile %q: %v", file, err)
			continue
		}

		keys = append(keys, path)
	}
	return keys
}

func parseKey(key keyConfig) (ssh.Signer, error) {
	if key.data == nil {
		bytes, err := os.ReadFile(key.path)
		if err != nil {
			if key.failOnNotExists || !os.IsNotExist(err) {
				return nil, fmt.Errorf("error reading public key file %q", key.path)
			}
			logging.V(5).Infof("[gitutils] Skipping non-existent public key file %q", key.path)

			return nil, nil
		}
		key.data = bytes
	}

	if len(key.data) == 0 {
		return nil, errors.New("empty key data")
	}

	signer, err := ssh.ParsePrivateKey(key.data)
	if err == nil {
		return signer, nil
	}

	if _, ok := err.(*ssh.PassphraseMissingError); !ok {
		return nil, err
	}

	signer, err = ssh.ParsePrivateKeyWithPassphrase(key.data, []byte(key.passphrase))
	if err == nil {
		return signer, nil
	}

	if key.failOnPassphrase {
		return nil, fmt.Errorf("Failed to parse private key file %q with provided passphrase: %v", key.path, err)
	}

	logging.V(3).Infof("[gitutils] Skipping private key file that requires passphrase %q", key.path)
	return nil, nil
}

// expandHomeDir expands file paths relative to the user's home directory (~) into absolute paths.
func expandHomeDir(path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	if path[0] != '~' {
		// Not a "~/foo" path.
		return path, nil
	}

	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		// We won't expand "~user"-style paths.
		return "", errors.New("cannot expand user-specific home dir")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, path[1:]), nil
}
