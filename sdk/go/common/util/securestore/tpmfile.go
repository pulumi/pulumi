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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/pulumihome"
)

// Never change: encrypted files reference this exact location.
const tpmFileName = "credentials-key.tpm"

// Only ever paired with tpmWrapper: the TPM binding, not the 0600 mode, is
// what makes a harvested file useless.
type tpmFileStore struct {
	path string
	err  error // non-nil when the Pulumi home directory could not be determined
}

// newTPMFileStore locates the key file under the Pulumi home directory.
// Deliberately pulumihome.Dir, not a workspace path helper: the key file's
// location must be stable, never redirected to a per-session agent directory.
// A home directory that cannot be determined disables only this backend, with
// available() reporting the cause.
func newTPMFileStore() itemStore {
	home, err := pulumihome.Dir()
	if err != nil {
		return tpmFileStore{err: err}
	}
	return tpmFileStore{path: filepath.Join(home, tpmFileName)}
}

// Proves writability with a probe file, never touching the item.
func (s tpmFileStore) available() (Outcome, error) {
	if err := s.probeWritable(); err != nil {
		return Absent, fmt.Errorf("%w: TPM key file location is not usable: %v", ErrUnavailable, err)
	}
	return Ready, nil
}

func (s tpmFileStore) probeWritable() error {
	if s.err != nil {
		return s.err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	probe, err := os.CreateTemp(dir, ".credentials-key-probe-*")
	if err != nil {
		return err
	}
	// Remove even when Close fails so no probe file is left behind.
	return errors.Join(probe.Close(), os.Remove(probe.Name()))
}

func (s tpmFileStore) get() (string, error) {
	if s.err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, s.err)
	}
	data, err := os.ReadFile(s.path)
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
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// Temp file + rename so a crash cannot truncate the item. CreateTemp
	// already gives mode 0600.
	tmp, err := os.CreateTemp(dir, ".credentials-key-*")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(value); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	//nolint:forbidigo // Same-directory rename
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

func (s tpmFileStore) delete() error {
	if s.err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, s.err)
	}
	err := os.Remove(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
