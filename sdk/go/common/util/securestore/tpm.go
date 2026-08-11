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

//go:build linux || windows

package securestore

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpm2/transport"
)

// A discrete TPM chip can take hundreds of milliseconds per command, wrap and
// unwrap issue several, and other TPM clients' commands may be queued ahead
// of ours. Generous, but still bounds a hung device.
const tpmOpTimeout = 10 * time.Second

// tpmWrapper is a keyWrapper that seals the key to the machine's TPM 2.0, so
// the persisted item is a sealed blob that is useless off the originating
// machine. The blob is created under a deterministic ECC-P256 storage primary
// in the owner hierarchy; the primary is regenerated from the hierarchy seed
// on every operation, so nothing is persisted inside the TPM itself and no
// TPM NV space or persistent handles are consumed.
//
// The transport is opened per operation and closed before returning, and
// every operation is bounded by withTimeout(tpmOpTimeout, ...).
type tpmWrapper struct{}

func (tpmWrapper) kind() wrapKind { return wrapTPM }

// available reports whether a TPM 2.0 is usable right now. Opening the
// transport is the real probe; the cheap GetCapability call additionally
// proves the device answers TPM 2.0 commands.
func (tpmWrapper) available() error {
	_, err := withTimeout(tpmOpTimeout, func() (struct{}, error) {
		tpm, err := openTPM()
		if err != nil {
			return struct{}{}, err
		}
		defer tpm.Close()
		_, err = tpm2.GetCapability{
			Capability:    tpm2.TPMCapTPMProperties,
			Property:      uint32(tpm2.TPMPTManufacturer),
			PropertyCount: 1,
		}.Execute(tpm)
		return struct{}{}, err
	})
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return err
		}
		return fmt.Errorf("%w: no usable TPM: %v", ErrUnavailable, err)
	}
	return nil
}

// sealedDataTemplate is the public template for the keyedhash SEALED-DATA
// object holding the key: fixed to this TPM and parent, no signing/decryption
// scheme, and userWithAuth with an empty auth value so Unseal is prompt-free.
// noDA is deliberately left false so failed unseal attempts count toward the
// TPM's dictionary-attack protection.
//
// The empty auth is deliberate for this silent tier; a future PIN-protected
// presence mode would set a real authValue (and revisit DA settings) so
// unsealing requires user interaction.
func sealedDataTemplate() tpm2.TPMTPublic {
	return tpm2.TPMTPublic{
		Type:    tpm2.TPMAlgKeyedHash,
		NameAlg: tpm2.TPMAlgSHA256,
		ObjectAttributes: tpm2.TPMAObject{
			FixedTPM:     true,
			FixedParent:  true,
			UserWithAuth: true,
		},
	}
}

// createStoragePrimary creates the storage primary in the owner hierarchy
// from the TCG reference ECC-P256 SRK template (restricted decrypt,
// AES-128-CFB symmetric). Primaries are derived deterministically from the
// hierarchy's fixed seed, so the same template yields the same key on every
// call as long as the TPM is not cleared.
func createStoragePrimary(tpm transport.TPM) (*tpm2.CreatePrimaryResponse, error) {
	rsp, err := tpm2.CreatePrimary{
		PrimaryHandle: tpm2.TPMRHOwner,
		InPublic:      tpm2.New2B(tpm2.ECCSRKTemplate),
	}.Execute(tpm)
	if err != nil {
		return nil, fmt.Errorf("creating TPM storage primary: %w", err)
	}
	return rsp, nil
}

// flushHandle releases a transient TPM handle. Failures are deliberately
// ignored: the transport is closed right afterwards, and both the Linux
// kernel resource manager (/dev/tpmrm0) and the Windows TBS reclaim a
// client's transient objects on close.
func flushHandle(tpm transport.TPM, h tpm2.TPMHandle) {
	_, _ = tpm2.FlushContext{FlushHandle: h}.Execute(tpm)
}

// wrap seals the key into a TPM-bound blob: CreatePrimary regenerates the
// storage primary, Create seals the key under it as a keyedhash SEALED-DATA
// object, and the resulting private+public blobs are serialized with
// encodeSealedBlob. Only the TPM that created the blob can unseal it.
func (tpmWrapper) wrap(key []byte) ([]byte, error) {
	return withTimeout(tpmOpTimeout, func() ([]byte, error) {
		tpm, err := openTPM()
		if err != nil {
			return nil, fmt.Errorf("%w: no usable TPM: %v", ErrUnavailable, err)
		}
		defer tpm.Close()

		primary, err := createStoragePrimary(tpm)
		if err != nil {
			return nil, err
		}
		defer flushHandle(tpm, primary.ObjectHandle)

		rsp, err := tpm2.Create{
			ParentHandle: tpm2.AuthHandle{
				Handle: primary.ObjectHandle,
				Name:   primary.Name,
				Auth:   tpm2.PasswordAuth(nil),
			},
			InSensitive: tpm2.TPM2BSensitiveCreate{
				Sensitive: &tpm2.TPMSSensitiveCreate{
					Data: tpm2.NewTPMUSensitiveCreate(&tpm2.TPM2BSensitiveData{Buffer: key}),
				},
			},
			InPublic: tpm2.New2B(sealedDataTemplate()),
		}.Execute(tpm)
		if err != nil {
			return nil, fmt.Errorf("sealing key in TPM: %w", err)
		}
		return encodeSealedBlob(tpm2.Marshal(rsp.OutPrivate), tpm2.Marshal(rsp.OutPublic))
	})
}

// unwrap recovers the key from a blob produced by wrap: it regenerates the
// same storage primary, loads the sealed object under it, and unseals it.
func (tpmWrapper) unwrap(blob []byte) ([]byte, error) {
	privBytes, pubBytes, err := decodeSealedBlob(blob)
	if err != nil {
		return nil, err
	}
	priv, err := tpm2.Unmarshal[tpm2.TPM2BPrivate](privBytes)
	if err != nil {
		return nil, fmt.Errorf("stored key is corrupt (bad TPM private blob): %w", err)
	}
	pub, err := tpm2.Unmarshal[tpm2.TPM2BPublic](pubBytes)
	if err != nil {
		return nil, fmt.Errorf("stored key is corrupt (bad TPM public blob): %w", err)
	}

	return withTimeout(tpmOpTimeout, func() ([]byte, error) {
		tpm, err := openTPM()
		if err != nil {
			return nil, fmt.Errorf("%w: no usable TPM: %v", ErrUnavailable, err)
		}
		defer tpm.Close()

		primary, err := createStoragePrimary(tpm)
		if err != nil {
			return nil, err
		}
		defer flushHandle(tpm, primary.ObjectHandle)

		loadRsp, err := tpm2.Load{
			ParentHandle: tpm2.AuthHandle{
				Handle: primary.ObjectHandle,
				Name:   primary.Name,
				Auth:   tpm2.PasswordAuth(nil),
			},
			InPrivate: *priv,
			InPublic:  *pub,
		}.Execute(tpm)
		if err != nil {
			return nil, fmt.Errorf("the TPM cannot load the stored key, "+
				"usually because the machine changed or its TPM was cleared: %w", err)
		}
		defer flushHandle(tpm, loadRsp.ObjectHandle)

		unsealRsp, err := tpm2.Unseal{
			ItemHandle: tpm2.AuthHandle{
				Handle: loadRsp.ObjectHandle,
				Name:   loadRsp.Name,
				Auth:   tpm2.PasswordAuth(nil),
			},
		}.Execute(tpm)
		if err != nil {
			return nil, fmt.Errorf("the TPM cannot unseal the stored key, "+
				"usually because the machine changed or its TPM was cleared: %w", err)
		}
		return unsealRsp.OutData.Buffer, nil
	})
}
