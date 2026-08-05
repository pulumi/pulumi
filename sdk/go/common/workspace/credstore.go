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

package workspace

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/securestore"
)

func credentialStoreMode() (securestore.Mode, error) {
	value := env.CredentialStore.Value()
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return securestore.ModeDefault, nil
	case "auto":
		return securestore.ModeAuto, nil
	case "os":
		return securestore.ModeOS, nil
	case "plaintext":
		return securestore.ModePlaintext, nil
	default:
		return 0, fmt.Errorf("invalid PULUMI_CREDENTIAL_STORE value %q (want auto, os, or plaintext)", value)
	}
}

// Memoized: probing costs an exec of /usr/bin/security on macOS.
var writeStore = sync.OnceValues(resolveWriteStore)

// Tests that change PULUMI_CREDENTIAL_STORE or install the mock must call this.
func resetWriteStoreForTesting() {
	writeStore = sync.OnceValues(resolveWriteStore)
	replacedEnvelope.Store(false)
	plaintextPendingOnce = sync.Once{}
}

func resolveWriteStore() (keyStore, error) {
	mode, err := credentialStoreMode()
	if err != nil {
		return nil, err
	}
	return stores.Resolve(mode)
}

// Deliberately does not record the backend: a stale decision could downgrade
// a user who has protection available.
type credStoreState struct {
	Warned bool `json:"warned,omitempty"`
}

func credStoreStatePath() (string, error) {
	credsFile, err := getCredsFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(credsFile), "credstore.json"), nil
}

func readCredStoreState() credStoreState {
	var state credStoreState
	path, err := credStoreStatePath()
	if err != nil {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	_ = json.Unmarshal(data, &state)
	return state
}

func writeCredStoreState(state credStoreState) {
	path, err := credStoreStatePath()
	if err != nil {
		return
	}
	data, err := json.Marshal(state)
	if err != nil {
		return
	}
	// Best effort: losing this file only means the warning shows once more.
	_ = os.WriteFile(path, data, 0o600)
}

var warnWriter io.Writer = os.Stderr

// Credentials are read by the engine and every language host alike.
var plaintextPendingOnce sync.Once

func warnPlaintextPending() {
	if !securestore.Attended() {
		return
	}
	plaintextPendingOnce.Do(func() {
		fmt.Fprintf(warnWriter,
			"warning: credentials are stored in plaintext; the next `pulumi login` or credential update will encrypt them\n")
	})
}

// Once per machine, and only when someone is watching.
func warnPlaintextFallback(reason error) {
	if !securestore.Attended() {
		logging.V(7).Infof("credential store unavailable, using plaintext: %v", reason)
		return
	}
	state := readCredStoreState()
	if state.Warned {
		return
	}
	fmt.Fprintf(warnWriter,
		"warning: storing Pulumi credentials in plaintext: %v\n"+
			"         Set PULUMI_CREDENTIAL_STORE=os to require OS-protected storage,\n"+
			"         or PULUMI_CREDENTIAL_STORE=plaintext to silence this warning.\n",
		reason)
	state.Warned = true
	writeCredStoreState(state)
}
