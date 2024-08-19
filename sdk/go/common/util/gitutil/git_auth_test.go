package gitutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/nettest"
)

// mockSSHConfig allows tests to mock SSH key paths.
type mockSSHConfig struct {
	paths []string
	err   error
}

var _ sshUserSettings = &mockSSHConfig{}

// GetKeyPath returns a canned response for SSH config.
func (c *mockSSHConfig) GetAllStrict(host, key string) ([]string, error) {
	return c.paths, c.err
}

type mockSSHAgentBroker struct {
	available    bool
	newError     error
	signers      []ssh.Signer
	signersError error
	closeError   error
}

var (
	_ sshAgentBroker = (*mockSSHAgentBroker)(nil)
	_ sshAgentImpl   = (*mockSSHAgentBroker)(nil)
)

// Available implements sshAgentBroker.
func (m *mockSSHAgentBroker) Available() bool {
	return m.available
}

// New implements sshAgentBroker.
func (m *mockSSHAgentBroker) New() (sshAgentImpl, error) {
	if m.newError != nil {
		return nil, m.newError
	}

	return m, nil
}

// Close implements sshAgentImpl.
func (m *mockSSHAgentBroker) Close() error {
	return m.closeError
}

// Signers implements sshAgentImpl.
func (m *mockSSHAgentBroker) Signers() ([]ssh.Signer, error) {
	if m.signersError != nil {
		return nil, m.signersError
	}
	return m.signers, nil
}

func captureStderr(t *testing.T, f func()) (string, error) {
	old := os.Stderr // keep backup of the real stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		require.NoError(t, err)
		outC <- buf.String()
	}()

	prevLog := logging.LogToStderr
	prevV := logging.Verbose
	prevFlow := logging.LogFlow
	logging.InitLogging(true, 9, true)

	// calling function which stderr we are going to capture:
	f()

	logging.InitLogging(prevLog, prevV, prevFlow)

	// back to normal state
	w.Close()
	os.Stderr = old // restoring the real stderr
	return <-outC, nil
}

func setupSSHAuthSocket(t *testing.T) {
	l, err := nettest.NewLocalListener("unix")
	t.Cleanup(func() {
		l.Close()
	})
	require.NoError(t, err)
	t.Setenv("SSH_AUTH_SOCK", l.Addr().String())
}

//nolint:paralleltest // uses mutable state
func TestNewDefaultSSHAuth(t *testing.T) {
	auth := NewDefaultSSHAuth("user")
	require.NotNil(t, auth)
	require.Len(t, auth.signers, 0)
}

type sshConfigMock struct {
	paths []string
}

var _ sshUserSettings = &sshConfigMock{}

func (c *sshConfigMock) GetAllStrict(host string, key string) ([]string, error) {
	if host == "*" && key == "IdentityFile" {
		return c.paths, nil
	}
	return nil, errors.New("Invalid key")
}

func mustExpandHomeDir(path string) string {
	expanded, err := expandHomeDir(path)
	if err != nil {
		panic(err)
	}
	return expanded
}

//nolint:paralleltest // uses mutable state
func TestDefaultIdentityFiles(t *testing.T) {
	keys := identityFiles(&sshConfigMock{
		paths: []string{"~/.ssh/identity"},
	}, "*")

	require.Len(t, keys, 4)

	require.ElementsMatch(t, keys, []string{
		mustExpandHomeDir("~/.ssh/identity"),
		mustExpandHomeDir("~/.ssh/id_rsa"),
		mustExpandHomeDir("~/.ssh/id_ecdsa"),
		mustExpandHomeDir("~/.ssh/id_ed25519"),
	})
}

func TestAddIdentityFiles(t *testing.T) {
	t.Parallel()

	// We're just reimplementing the logic and manually counting the files that exist, verifying that
	// we found public keys for them.
	//
	// The libraries we use make testing this a bit unfortunate, as they hardcode using the user's
	// home dir, and for safety reasons we do not want to write (even temporarily) key data in tests
	// to a user's home dir.
	t.Run("Uses Local IdentityFiles", func(t *testing.T) {
		t.Parallel()

		auth := NewDefaultSSHAuth("user")
		require.NotNil(t, auth)

		err := auth.AddIdentityFiles("*", &sshConfigMock{
			paths: []string{
				generateSSHKeyFile(t, "good"),
				generateSSHKeyFile(t, "good"),
				generateSSHKeyFile(t, "good"),
				generateSSHKeyFile(t, "good"),
				generateSSHKeyFile(t, "bad"),
				generateSSHKeyFile(t, "bad"),
				generateSSHKeyFile(t, "bad"),
				"/err/invalid-path",
			},
		}, "good")
		require.NoError(t, err)
		require.Len(t, auth.signers, 4)
	})
}

//nolint:paralleltest // uses mutable state
func TestBasicAddKeys(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")

			signerCount := len(auth.signers)

			keyFile := generateSSHKeyFile(t, "")
			err := auth.AddKeyFiles(keyFile)
			require.NoError(t, err)
			require.NotNil(t, auth)
			require.Len(t, auth.signers, signerCount+1)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "Adding SSH public key file")
	})

	t.Run("InvalidKeyFile", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			err := auth.AddKeyFiles("nonexistentfile")
			require.Error(t, err)
		})
		require.NoError(t, err)
		require.Empty(t, logs)
	})
}

//nolint:paralleltest // uses mutable state
func TestNewDefaultSSHAuthFromFilesWithPassphrase(t *testing.T) {
	t.Run("WithValidKey", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			keyFile := generateSSHKeyFile(t, "passphrase")
			err := auth.AddKeyFilesWithPassphrase([]string{keyFile}, "passphrase")
			require.NoError(t, err)
			require.NotNil(t, auth)

			require.Len(t, auth.signers, 1)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "Adding SSH public key file with passphrase")
	})

	t.Run("WithInvalidPassphrase", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			keyFile := generateSSHKeyFile(t, "good-passphrase")
			err := auth.AddKeyFilesWithPassphrase([]string{keyFile}, "bad-passphrase")
			require.Error(t, err)

			require.Len(t, auth.signers, 0)
		})
		require.NoError(t, err)
		require.Empty(t, logs)
	})

	t.Run("PassphrasedWithNoPassphrase", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			keyFile := generateSSHKeyFile(t, "good-passphrase")
			err := auth.AddKeyFiles(keyFile)
			require.NoError(t, err)

			require.Len(t, auth.signers, 0)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "Skipping private key file that requires passphrase")
	})

	t.Run("WithValidPassphrase", func(t *testing.T) {
		auth := NewDefaultSSHAuth("user")
		require.Len(t, auth.signers, 0)

		keyFile := generateSSHKeyFile(t, "correctpassphrase")
		err := auth.AddKeyFilesWithPassphrase([]string{keyFile}, "correctpassphrase")
		require.NoError(t, err)
		require.NotNil(t, auth)
		require.Len(t, auth.signers, 1)
	})
}

//nolint:paralleltest // uses mutable state
func TestNewDefaultSSHAuthFromPEMWithPassphrase(t *testing.T) {
	t.Run("WithValidKey", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			data := generateSSHKeyPEM(t, "passphrase")
			err := auth.AddKeyPemWithPassphrase([][]byte{data}, "passphrase")
			require.NoError(t, err)
			require.NotNil(t, auth)

			require.Len(t, auth.signers, 1)
		})
		require.NoError(t, err)
		require.Empty(t, logs)
	})

	t.Run("WithInvalidPassphrase", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			data := generateSSHKeyPEM(t, "good-passphrase")
			err := auth.AddKeyPemWithPassphrase([][]byte{data}, "bad-passphrase")
			require.Error(t, err)

			require.Len(t, auth.signers, 0)
		})
		require.NoError(t, err)
		require.Empty(t, logs)
	})

	t.Run("PassphrasedWithNoPassphrase", func(t *testing.T) {
		logs, err := captureStderr(t, func() {
			auth := NewDefaultSSHAuth("user")
			require.Len(t, auth.signers, 0)

			data := generateSSHKeyPEM(t, "good-passphrase")
			err := auth.AddKeyPem(data)
			require.NoError(t, err)

			require.Len(t, auth.signers, 0)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "Skipping private key file that requires passphrase")
	})

	t.Run("WithValidPassphrase", func(t *testing.T) {
		auth := NewDefaultSSHAuth("user")
		require.Len(t, auth.signers, 0)

		data := generateSSHKeyPEM(t, "correctpassphrase")
		err := auth.AddKeyPemWithPassphrase([][]byte{data}, "correctpassphrase")
		require.NoError(t, err)
		require.NotNil(t, auth)
		require.Len(t, auth.signers, 1)
	})
}

//nolint:paralleltest // uses mutable state
func TestNewDefaultSSHAuthFromAgent(t *testing.T) {
	t.Run("With SSHAgent", func(t *testing.T) {
		setupSSHAuthSocket(t)
		auth := NewDefaultSSHAuth("user")

		signer, err := parseKey(keyConfig{path: generateSSHKeyFile(t, "")})
		require.NoError(t, err)

		err = auth.AddSSHAgentBroker(&mockSSHAgentBroker{
			available: true,
			signers:   []ssh.Signer{signer},
		})
		require.NoError(t, err)

		publicKeys, err := auth.publicKeys()
		require.NoError(t, err)
		require.Len(t, publicKeys, 1)
	})

	t.Run("Without SSH Agent", func(t *testing.T) {
		setupSSHAuthSocket(t)
		auth := NewDefaultSSHAuth("user")

		signer, err := parseKey(keyConfig{path: generateSSHKeyFile(t, "")})
		require.NoError(t, err)

		logs, err := captureStderr(t, func() {
			err = auth.AddSSHAgentBroker(&mockSSHAgentBroker{
				available: false,
				signers:   []ssh.Signer{signer},
			})
			require.NoError(t, err)

			publicKeys, err := auth.publicKeys()
			require.NoError(t, err)
			require.Len(t, publicKeys, 0)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "SSH agent not available")
	})

	t.Run("Erroring SSH Agent on Init", func(t *testing.T) {
		setupSSHAuthSocket(t)
		auth := NewDefaultSSHAuth("user")

		signer, err := parseKey(keyConfig{path: generateSSHKeyFile(t, "")})
		require.NoError(t, err)

		logs, err := captureStderr(t, func() {
			err = auth.AddSSHAgentBroker(&mockSSHAgentBroker{
				available: true,
				signers:   []ssh.Signer{signer},
				newError:  errors.New("error ssh-sock not available [mock]"),
			})
			require.NoError(t, err)

			publicKeys, err := auth.publicKeys()
			require.NoError(t, err)
			require.Len(t, publicKeys, 0)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "error ssh-sock not available [mock]")
	})

	t.Run("Erroring SSH Agent on Use", func(t *testing.T) {
		setupSSHAuthSocket(t)
		auth := NewDefaultSSHAuth("user")

		signer, err := parseKey(keyConfig{path: generateSSHKeyFile(t, "")})
		require.NoError(t, err)

		logs, err := captureStderr(t, func() {
			err = auth.AddSSHAgentBroker(&mockSSHAgentBroker{
				available:    true,
				signers:      []ssh.Signer{signer},
				signersError: errors.New("error SSH agent invalid socket [mock]"),
			})
			require.NoError(t, err)

			publicKeys, err := auth.publicKeys()
			require.NoError(t, err)
			require.Len(t, publicKeys, 0)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "error SSH agent invalid socket [mock]")
	})

	t.Run("Erroring SSH Agent on Close", func(t *testing.T) {
		setupSSHAuthSocket(t)
		auth := NewDefaultSSHAuth("user")

		signer, err := parseKey(keyConfig{path: generateSSHKeyFile(t, "")})
		require.NoError(t, err)

		logs, err := captureStderr(t, func() {
			err = auth.AddSSHAgentBroker(&mockSSHAgentBroker{
				available:  true,
				signers:    []ssh.Signer{signer},
				closeError: errors.New("error ssh-sock failed to close [mock]"),
			})
			require.NoError(t, err)

			publicKeys, err := auth.publicKeys()
			require.NoError(t, err)
			require.Len(t, publicKeys, 1)
		})
		require.NoError(t, err)
		require.Contains(t, logs, "error ssh-sock failed to close [mock]")
	})
}

func generateSSHKeyBlock(t *testing.T, passphrase string) *pem.Block {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	if passphrase != "" {
		//nolint: staticcheck
		block, err = x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(passphrase), x509.PEMCipherAES256)
		require.NoError(t, err)
	}

	return block
}

func generateSSHKeyFile(t *testing.T, passphrase string) string {
	block := generateSSHKeyBlock(t, passphrase)

	path := filepath.Join(t.TempDir(), "test-key")
	err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600)
	t.Cleanup(func() {
		os.Remove(path)
	})
	require.NoError(t, err)

	return path
}

func generateSSHKeyPEM(t *testing.T, passphrase string) []byte {
	block := generateSSHKeyBlock(t, passphrase)

	return pem.EncodeToMemory(block)
}
