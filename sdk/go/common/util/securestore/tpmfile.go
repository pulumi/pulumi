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

// Never change: encrypted files reference this exact location.
const tpmFileName = "credentials-key.tpm"

// Only ever paired with tpmWrapper: the TPM binding, not the 0600 mode, is
// what makes a harvested file useless.
type tpmFileStore struct {
	path string
	err  error // non-nil when the Pulumi home directory could not be determined
}

// The caller resolves pulumiHome; this package cannot import workspace.
func newTPMFileStore(pulumiHome string) itemStore {
	if pulumiHome == "" {
		return tpmFileStore{err: errors.New("the Pulumi home directory could not be determined, set PULUMI_HOME")}
	}
	return tpmFileStore{path: filepath.Join(pulumiHome, tpmFileName)}
}

// Proves writability with a probe file, never touching the item.
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
		// Temp file + rename so a crash cannot truncate the item. CreateTemp
		// already gives mode 0600.
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
		// Same-directory rename, so //nolint:forbidigo below is safe.
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
