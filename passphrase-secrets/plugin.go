// Copyright 2016-2022, Pulumi Corporation.
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

package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	secretsrpc "github.com/pulumi/pulumi/sdk/v3/proto/go/secrets"
)

var ErrIncorrectPassphrase = errors.New("incorrect passphrase")

// given a passphrase and an encryption state, construct a Crypter from it. Our encryption
// state value is a version tag followed by version specific state information. Presently, we only have one version
// we support (`v1`) which is AES-256-GCM using a key derived from a passphrase using 1,000,000 iterations of PDKDF2
// using SHA256.
func symmetricCrypterFromPhraseAndState(ctx context.Context, phrase string, state string) (config.Crypter, error) {
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
	decrypted, err := decrypter.DecryptValue(ctx, splits[2])
	if err != nil || decrypted != "pulumi" {
		return nil, ErrIncorrectPassphrase
	}

	return decrypter, nil
}

type passphraseSecretsState struct {
	Salt string `json:"salt"`
}

func NewPassphraseSecretPlugin() secretsrpc.SecretsProviderServer {
	return &passphraseSecretPlugin{}
}

type passphraseSecretPlugin struct {
	crypter config.Crypter
}

func (p *passphraseSecretPlugin) Encrypt(ctx context.Context, req *secretsrpc.EncryptRequest) (*secretsrpc.EncryptResponse, error) {
	if p.crypter == nil {
		return nil, fmt.Errorf("plugin not setup")
	}

	ciphertexts := make([]string, len(req.Plaintexts))
	for i, plaintext := range req.Plaintexts {
		ciphertext, err := p.crypter.EncryptValue(ctx, plaintext)
		if err != nil {
			return nil, err
		}
		ciphertexts[i] = ciphertext
	}

	return &secretsrpc.EncryptResponse{
		Ciphertexts: ciphertexts,
	}, nil
}

func (p *passphraseSecretPlugin) Decrypt(ctx context.Context, req *secretsrpc.DecryptRequest) (*secretsrpc.DecryptResponse, error) {
	if p.crypter == nil {
		return nil, fmt.Errorf("plugin not setup")
	}

	plaintexts := make([]string, len(req.Ciphertexts))
	for i, ciphertext := range req.Ciphertexts {
		plaintext, err := p.crypter.DecryptValue(ctx, ciphertext)
		if err != nil {
			return nil, err
		}
		plaintexts[i] = plaintext
	}

	return &secretsrpc.DecryptResponse{
		Plaintexts: plaintexts,
	}, nil
}

func readPassphrase() (*string, error) {
	if phrase, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE"); ok {
		return &phrase, nil
	}
	if phraseFile, ok := os.LookupEnv("PULUMI_CONFIG_PASSPHRASE_FILE"); ok && phraseFile != "" {
		phraseFilePath, err := filepath.Abs(phraseFile)
		if err != nil {
			return nil, fmt.Errorf("unable to construct a path the PULUMI_CONFIG_PASSPHRASE_FILE: %w", err)
		}
		phraseDetails, err := os.ReadFile(phraseFilePath)
		if err != nil {
			return nil, fmt.Errorf("unable to read PULUMI_CONFIG_PASSPHRASE_FILE: %w", err)
		}
		phrase := strings.TrimSpace(string(phraseDetails))
		return &phrase, nil
	}
	return nil, nil
}

func (p *passphraseSecretPlugin) Configure(ctx context.Context, req *secretsrpc.ConfigureRequest) (*secretsrpc.ConfigureResponse, error) {
	if p.crypter != nil {
		return nil, fmt.Errorf("already setup")
	}

	var s passphraseSecretsState
	if err := json.Unmarshal([]byte(req.State), &s); err != nil {
		return nil, fmt.Errorf("unmarshalling state: %w", err)
	}

	// If we've been given a passphrase input use it
	var passphrase *string
	if input, has := req.Inputs["passphrase"]; has {
		passphrase = &input
	} else {
		// Else try to read from the environment or file
		var err error
		passphrase, err = readPassphrase()
		if err != nil {
			return nil, err
		}
	}
	const promptMsg = "Enter your passphrase to unlock config/secrets " +
		"(set PULUMI_CONFIG_PASSPHRASE or PULUMI_CONFIG_PASSPHRASE_FILE to remember)"
	// If we weren't given the passphrase, and couldn't read it from the environment then prompt for it.
	if passphrase == nil {
		const errMsg = "passphrase must be set with PULUMI_CONFIG_PASSPHRASE or " +
			"PULUMI_CONFIG_PASSPHRASE_FILE environment variables"
		return &secretsrpc.ConfigureResponse{
			Prompt: &secretsrpc.Prompt{
				Label: "passphrase",
				Text:  promptMsg,
				Error: errMsg,
			},
		}, nil
	}

	crypter, err := symmetricCrypterFromPhraseAndState(ctx, *passphrase, s.Salt)
	if err == ErrIncorrectPassphrase {
		if _, has := req.Inputs["passphrase"]; has {
			// If the user sent this as a prompt response then we should return a new prompt informing them the error was wrong
			return &secretsrpc.ConfigureResponse{
				Prompt: &secretsrpc.Prompt{
					Label: "passphrase",
					Text:  promptMsg,
					Error: err.Error(),
				},
			}, nil
		}
		// Else just return the error straight, we're not using prompts (i.e. we read via the envvar and we
		// don't fall back to prompting when that's set)
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	p.crypter = crypter
	return &secretsrpc.ConfigureResponse{}, nil
}

func (p *passphraseSecretPlugin) Initialize(ctx context.Context, req *secretsrpc.InitializeRequest) (*secretsrpc.InitializeResponse, error) {
	if p.crypter != nil {
		return nil, fmt.Errorf("already setup")
	}

	// We expect no arguments here
	if len(req.Args) != 0 {
		return nil, fmt.Errorf("expected zero arguments got %d", len(req.Args))
	}

	const promptMsg = "Enter your new passphrase to protect config/secrets"

	var passphrase *string
	if input, has := req.Inputs["passphrase2"]; has {
		// If we've been given two passphrase inputs check they match
		if input == req.Inputs["passphrase1"] {
			passphrase = &input
		} else {
			return &secretsrpc.InitializeResponse{
				Prompt: &secretsrpc.Prompt{
					Label: "passphrase1",
					Text:  promptMsg,
					Error: "passphrases do not match",
				},
			}, nil
		}
	} else if _, has := req.Inputs["passphrase1"]; has {
		// If we've been given a first passphrase input, confirm it
		return &secretsrpc.InitializeResponse{
			Prompt: &secretsrpc.Prompt{
				Label:    "passphrase2",
				Text:     "Re-enter your passphrase to confirm",
				Error:    "",
				Preserve: []string{"passphrase1"},
			},
		}, nil
	} else {
		// Else try to read from the environment or file
		var err error
		passphrase, err = readPassphrase()
		if err != nil {
			return nil, err
		}

		// If we weren't given the passphrase, and couldn't read it from the environment then prompt for it.
		if passphrase == nil {
			const errMsg = "passphrase must be set with PULUMI_CONFIG_PASSPHRASE or " +
				"PULUMI_CONFIG_PASSPHRASE_FILE environment variables"
			return &secretsrpc.InitializeResponse{
				Prompt: &secretsrpc.Prompt{
					Label: "passphrase1",
					Text:  promptMsg,
					Error: errMsg,
				},
			}, nil
		}
	}

	// Produce a new salt.
	salt := make([]byte, 8)
	_, err := cryptorand.Read(salt)
	if err != nil {
		return nil, fmt.Errorf("could not read from system random: %w", err)
	}

	// Encrypt a message and store it with the salt so we can test if the password is correct later.
	crypter := config.NewSymmetricCrypterFromPassphrase(*passphrase, salt)
	msg, err := crypter.EncryptValue(ctx, "pulumi")
	if err != nil {
		return nil, err
	}

	state := passphraseSecretsState{
		// Encode the salt as the passphrase secrets manager state.
		Salt: fmt.Sprintf("v1:%s:%s", base64.StdEncoding.EncodeToString(salt), msg),
	}

	jsonState, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	p.crypter = crypter
	return &secretsrpc.InitializeResponse{State: string(jsonState)}, nil
}
