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

// Tests that change PULUMI_CREDENTIAL_STORE or install the mock must call this.
func resetCredStoreForTesting() {
	replacedEnvelope.Store(false)
	plaintextPendingOnce = sync.Once{}
}

// Cheap to call repeatedly: the store probes memoize their own prechecks.
func resolveWriteStore() (keyStore, error) {
	mode, err := credentialStoreMode()
	if err != nil {
		return nil, err
	}
	return stores.Resolve(mode)
}

// Credentials are read by the engine and every language host alike.
var plaintextPendingOnce sync.Once

// The credentials file is plaintext and the user wants to use a secure store now, so warn that the
// next write will encrypt it.
func warnPlaintextPending() {
	if !securestore.Attended() {
		return
	}
	plaintextPendingOnce.Do(func() {
		st, err := resolveWriteStore()
		if err != nil || st.Backend() == securestore.BackendPlaintext {
			return
		}
		fmt.Fprintf(os.Stderr,
			"warning: credentials are stored in plaintext; the next `pulumi login` or credential update will encrypt them\n")
	})
}

// SuppressPlaintextPendingWarning silences the "the next `pulumi login` will
// encrypt them" warning for commands that perform that migration themselves.
func SuppressPlaintextPendingWarning() {
	plaintextPendingOnce.Do(func() {})
}

// Confirms the transition from plaintext to encrypted credentials.
func noteCredentialsEncrypted() {
	if !securestore.Attended() {
		return
	}
	fmt.Fprintln(os.Stderr, "Stored Pulumi credentials are now encrypted")
}

// Called when a write lands on plaintext unexpectedly: an available store
// failed its key operation, or a login recovery could not re-encrypt. Auto
// mode resolving to plaintext because no store exists stays quiet though.
func warnPlaintextFallback(reason error) {
	if !securestore.Attended() {
		logging.V(7).Infof("credential store unavailable, using plaintext: %v", reason)
		return
	}
	fmt.Fprintf(os.Stderr,
		"warning: storing Pulumi credentials in plaintext: %v\n"+
			"         Set PULUMI_CREDENTIAL_STORE=os to require OS-protected storage,\n"+
			"         or PULUMI_CREDENTIAL_STORE=plaintext to silence this warning.\n",
		reason)
}
