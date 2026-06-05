// Copyright 2026, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

type loggingCrypter struct{}

func (loggingCrypter) EncryptValue(ctx context.Context, plaintext string) (string, error) {
	logging.Infof("loggingCrypter: encrypting a value")
	return config.Base64Crypter.EncryptValue(ctx, plaintext)
}

func (loggingCrypter) BatchEncrypt(ctx context.Context, secrets []string) ([]string, error) {
	logging.Infof("loggingCrypter: batch encrypting %d values", len(secrets))
	return config.Base64Crypter.BatchEncrypt(ctx, secrets)
}

func (loggingCrypter) DecryptValue(ctx context.Context, ciphertext string) (string, error) {
	return config.Base64Crypter.DecryptValue(ctx, ciphertext)
}

func (loggingCrypter) BatchDecrypt(ctx context.Context, ciphertexts []string) ([]string, error) {
	return config.Base64Crypter.BatchDecrypt(ctx, ciphertexts)
}

type loggingSecretsManager struct{}

func (loggingSecretsManager) Type() string                { return "logging-test" }
func (loggingSecretsManager) State() json.RawMessage      { return json.RawMessage("{}") }
func (loggingSecretsManager) Encrypter() config.Encrypter { return loggingCrypter{} }
func (loggingSecretsManager) Decrypter() config.Decrypter { return loggingCrypter{} }

func TestUpgradeToEncryptedDoesNotDeadlock(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	l, err := StartLogging(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	const preUpgrade = "log line written before the upgrade"
	logging.Infof("%s", preUpgrade)

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					logging.Infof("concurrent log line")
				}
			}
		}()
	}

	done := make(chan error, 1)
	go func() {
		done <- l.UpgradeToEncrypted(t.Context(), "test-stack", "update-id", loggingSecretsManager{})
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		require.Fail(t, "UpgradeToEncrypted deadlocked (did not return within 15s).\n\nGoroutine dump:\n%s", buf[:n])
	}

	close(stop)
	wg.Wait()

	require.True(t, l.encrypted, "logger should be in encrypted mode after upgrade")

	// The upgraded log must still be a valid, decryptable PLOG file that
	// preserves data written before the upgrade — i.e. the fix doesn't just
	// avoid the deadlock, it produces a correct encrypted log.
	logPath := l.FilePath()
	require.NoError(t, l.Close())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer f.Close()

	r, err := encryptedlog.NewReader(t.Context(), f, config.Base64Crypter)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(r)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(plaintext), preUpgrade),
		"decrypted log should contain data written before the upgrade")
}

// TestRenameWhileWriting exercises renaming the live log file while concurrent
// writes are in flight, both before and after the upgrade to encrypted mode.
// On Windows a file open by the process cannot be renamed, so the rename must
// close and reopen the file underneath the active sink without losing data.
func TestRenameWhileWriting(t *testing.T) {
	t.Setenv("PULUMI_HOME", t.TempDir())

	l, err := StartLogging(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					logging.Infof("concurrent log line")
				}
			}
		}()
	}

	require.NoError(t, l.rename("gzip-stack", "gzip-update"))

	const preUpgrade = "log line written before the upgrade"
	logging.Infof("%s", preUpgrade)

	require.NoError(t, l.UpgradeToEncrypted(t.Context(), "enc-stack", "enc-update", loggingSecretsManager{}))
	require.True(t, l.encrypted, "logger should be in encrypted mode after upgrade")

	const postUpgrade = "log line written after the upgrade and before the rename"
	logging.Infof("%s", postUpgrade)

	require.NoError(t, l.rename("renamed-stack", "renamed-update"))
	require.Contains(t, l.FilePath(), "renamed-stack", "file should have been renamed")

	logging.Infof("log line written after the encrypted rename")

	close(stop)
	wg.Wait()

	logPath := l.FilePath()
	require.NoError(t, l.Close())

	f, err := os.Open(logPath)
	require.NoError(t, err)
	defer f.Close()

	r, err := encryptedlog.NewReader(t.Context(), f, config.Base64Crypter)
	require.NoError(t, err)
	plaintext, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Contains(t, string(plaintext), preUpgrade,
		"decrypted log should contain data written before the upgrade")
	require.Contains(t, string(plaintext), postUpgrade,
		"decrypted log should contain data written after the upgrade but before the rename")
}
