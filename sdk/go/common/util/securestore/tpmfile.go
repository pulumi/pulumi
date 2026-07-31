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
	"os"
	"path/filepath"
)

// tpmFileName is the file under the Pulumi home directory holding the item
// when no OS credential store is usable. Never change it: existing users'
// encrypted files reference this exact location.
const tpmFileName = "credentials-key.tpm"

// tpmFileStore is an itemStore persisting the item in a private file. It is
// only ever paired with tpmWrapper, so the file content is a TPM-sealed blob:
// the 0600 mode is defense in depth, while the TPM binding is what keeps a
// harvested file useless off the originating machine.
type tpmFileStore struct {
	path string
	err  error // non-nil when the Pulumi home directory could not be determined
}

// newTPMFileStore returns the itemStore keeping the TPM-sealed key in a file
// under the Pulumi home directory.
func newTPMFileStore() itemStore {
	dir, err := pulumiHomeDir()
	if err != nil {
		return tpmFileStore{err: err}
	}
	return tpmFileStore{path: filepath.Join(dir, tpmFileName)}
}

// pulumiHomeDir resolves $PULUMI_HOME, falling back to $HOME/.pulumi. This
// deliberately duplicates the tiny path rule of workspace.GetPulumiHomeDir:
// importing the workspace package from here would create an import cycle.
func pulumiHomeDir() (string, error) {
	if dir := os.Getenv("PULUMI_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not find user home directory, set PULUMI_HOME: %w", err)
	}
	return filepath.Join(home, ".pulumi"), nil
}

// available reports whether the key file's directory exists (creating it if
// needed) and is actually writable, proven by creating and removing a probe
// file without touching the real item.
func (s tpmFileStore) available() (Outcome, error) {
	_, err := withTimeout(func() (struct{}, error) {
		if s.err != nil {
			return struct{}{}, s.err
		}
		dir := filepath.Dir(s.path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return struct{}{}, err
		}
		probe, err := os.CreateTemp(dir, ".credentials-key-probe-*")
		if err != nil {
			return struct{}{}, err
		}
		if err := probe.Close(); err != nil {
			return struct{}{}, err
		}
		return struct{}{}, os.Remove(probe.Name())
	})
	if err != nil {
		if errors.Is(err, ErrUnavailable) {
			return Absent, err
		}
		return Absent, fmt.Errorf("%w: TPM key file location is not usable: %v", ErrUnavailable, err)
	}
	return Ready, nil
}

func (s tpmFileStore) get() (string, error) {
	if s.err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, s.err)
	}
	data, err := withTimeout(func() ([]byte, error) {
		return os.ReadFile(s.path)
	})
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s tpmFileStore) set(value string) error {
	if s.err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, s.err)
	}
	_, err := withTimeout(func() (struct{}, error) {
		dir := filepath.Dir(s.path)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return struct{}{}, err
		}
		// Write to a temp file and rename into place so a crash can never
		// leave a truncated item behind. os.CreateTemp creates the file with
		// mode 0600.
		tmp, err := os.CreateTemp(dir, ".credentials-key-*")
		if err != nil {
			return struct{}{}, err
		}
		if _, err := tmp.WriteString(value); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return struct{}{}, err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return struct{}{}, err
		}
		// Same-directory rename, mirroring workspace/creds.go's atomic write.
		if err := os.Rename(tmp.Name(), s.path); err != nil { //nolint:forbidigo
			os.Remove(tmp.Name())
			return struct{}{}, err
		}
		return struct{}{}, nil
	})
	return err
}

func (s tpmFileStore) delete() error {
	if s.err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, s.err)
	}
	_, err := withTimeout(func() (struct{}, error) {
		return struct{}{}, os.Remove(s.path)
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
