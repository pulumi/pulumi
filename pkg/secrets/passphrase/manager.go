// Copyright 2016-2019, Pulumi Corporation.
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
package passphrase

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/secrets"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

const Type = "passphrase"

var ErrIncorrectPassphrase = errors.New("incorrect passphrase")

// given a passphrase and an encryption state, construct a Crypter from it. Our encryption
// state value is a version tag followed by version specific state information. Presently, we only have one version
// we support (`v1`) which is AES-256-GCM using a key derived from a passphrase using 1,000,000 iterations of PDKDF2
// using SHA256.
func symmetricCrypterFromPhraseAndState(phrase string, state string) (config.Crypter, error) {
	splits := strings.SplitN(state, ":", 3)
	if len(splits) != 3 {
		return nil, errors.New("malformed state value")
	}

	if splits[0] != "v1" {
		return nil, errors.New("unknown state version")
	}

	salt, err := base64.StdEncoding.DecodeString(splits[1])
	if err != nil {
		return nil, err
	}

	decrypter := config.NewSymmetricCrypterFromPassphrase(phrase, salt)
	decrypted, err := decrypter.DecryptValue(state[indexN(state, ":", 2)+1:])
	if err != nil || decrypted != "pulumi" {
		return nil, ErrIncorrectPassphrase
	}

	return decrypter, nil
}

func indexN(s string, substr string, n int) int {
	contract.Require(n > 0, "n")
	scratch := s

	for i := n; i > 0; i-- {
		idx := strings.Index(scratch, substr)
		if i == -1 {
			return -1
		}

		scratch = scratch[idx+1:]
	}

	return len(s) - (len(scratch) + len(substr))
}

type localSecretsManagerState struct {
	Salt string `json:"salt"`
}

var _ secrets.Manager = &localSecretsManager{}

type localSecretsManager struct {
	state   localSecretsManagerState
	crypter config.Crypter
}

func (sm *localSecretsManager) Type() string {
	return Type
}

func (sm *localSecretsManager) State() interface{} {
	return sm.state
}

func (sm *localSecretsManager) Decrypter() (config.Decrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func (sm *localSecretsManager) Encrypter() (config.Encrypter, error) {
	contract.Assert(sm.crypter != nil)
	return sm.crypter, nil
}

func NewPassphaseSecretsManager(phrase string, state string) (secrets.Manager, error) {
	crypter, err := symmetricCrypterFromPhraseAndState(phrase, state)
	if err != nil {
		return nil, err
	}

	return &localSecretsManager{
		crypter: crypter,
		state: localSecretsManagerState{
			Salt: state,
		},
	}, nil
}

type provider struct{}

var _ secrets.ManagerProvider = &provider{}

func NewProvider() secrets.ManagerProvider {
	return &provider{}
}

func (p *provider) FromState(state json.RawMessage) (secrets.Manager, error) {
	var s localSecretsManagerState
	if err := json.Unmarshal(state, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshalling state")
	}

	// This is not ideal, but we don't have a great way to prompt the user in this case, since this may be
	// called during an update when trying to read stack outputs as part servicing a StackReference request
	// (since we need to decrypt the deployment)
	phrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE")

	sm, err := NewPassphaseSecretsManager(phrase, s.Salt)
	switch {
	case err == ErrIncorrectPassphrase:
		return nil, errors.New("incorrect passphrase, please set PULUMI_CONFIG_PASSPHRASE to the correct passphrase")
	case err != nil:
		return nil, errors.Wrap(err, "constructing secrets manager")
	default:
		return sm, nil
	}
}
