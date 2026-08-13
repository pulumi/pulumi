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
	"fmt"
	"io"
	"os"
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
	encryptedNoteOnce = sync.Once{}
}

func resolveWriteStore() (keyStore, error) {
	mode, err := credentialStoreMode()
	if err != nil {
		return nil, err
	}
	return stores.Resolve(mode)
}

var warnWriter io.Writer = os.Stderr

// Credentials are read by the engine and every language host alike.
var plaintextPendingOnce sync.Once

func warnPlaintextPending() {
	if !securestore.Attended() {
		return
	}
	// Encryption must actually be coming: with no usable store, the next
	// login falls back to plaintext rather than encrypt. The probe behind
	// writeStore is memoized, and only reads of a plaintext file in an
	// opted-in mode reach here.
	if st, err := writeStore(); err != nil || st.Backend() == securestore.BackendPlaintext {
		return
	}
	plaintextPendingOnce.Do(func() {
		fmt.Fprintf(warnWriter,
			"warning: credentials are stored in plaintext; the next `pulumi login` or credential update will encrypt them\n")
	})
}

// SuppressPlaintextPendingWarning silences the "the next `pulumi login` will
// encrypt them" warning for commands that perform that migration themselves.
func SuppressPlaintextPendingWarning() {
	plaintextPendingOnce.Do(func() {})
}

var encryptedNoteOnce sync.Once

// Confirms the migration the pending warning promised.
func noteCredentialsEncrypted() {
	if !securestore.Attended() {
		return
	}
	encryptedNoteOnce.Do(func() {
		fmt.Fprintf(warnWriter, "Stored Pulumi credentials are now encrypted\n")
	})
}

// Callers invoke this only for a write that transitions the user into
// plaintext (no plaintext file existed before), and only when someone is
// watching. Rewrites of an already-plaintext file stay quiet, so nobody is
// nagged on every credential refresh — no persisted warned-once state needed.
func warnPlaintextFallback(reason error) {
	if !securestore.Attended() {
		logging.V(7).Infof("credential store unavailable, using plaintext: %v", reason)
		return
	}
	fmt.Fprintf(warnWriter,
		"warning: storing Pulumi credentials in plaintext: %v\n"+
			"         Set PULUMI_CREDENTIAL_STORE=os to require OS-protected storage,\n"+
			"         or PULUMI_CREDENTIAL_STORE=plaintext to silence this warning.\n",
		reason)
}
