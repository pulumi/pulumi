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

//go:build linux

package securestore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/godbus/dbus/v5"
)

const (
	// secretServiceBusName is the well-known D-Bus name of the Secret
	// Service (gnome-keyring, KWallet 5.97+, KeePassXC, ...).
	secretServiceBusName = "org.freedesktop.secrets"
	// defaultCollectionPath is the alias object for the user's default
	// secret collection.
	defaultCollectionPath = dbus.ObjectPath("/org/freedesktop/secrets/aliases/default")
	// collectionLockedProperty is the standard Locked property on a
	// collection.
	collectionLockedProperty = "org.freedesktop.Secret.Collection.Locked"
)

// secretServicePrecheck fast-fails before go-keyring ever touches the Secret
// Service, guaranteeing the probe is prompt-free: it verifies a session bus
// exists, that a Secret Service provider is actually running, and that the
// default collection is unlocked. It never calls Unlock — unlocking may pop
// an OS dialog, which this package must never trigger.
func secretServicePrecheck() error {
	// Cheap environment check first: no session bus address and no
	// user-runtime bus socket means connecting is pointless.
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return fmt.Errorf("%w: no D-Bus session bus", ErrUnavailable)
		}
		if _, err := os.Stat(filepath.Join(runtimeDir, "bus")); err != nil {
			return fmt.Errorf("%w: no D-Bus session bus", ErrUnavailable)
		}
	}

	_, err := withTimeout(func() (struct{}, error) {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			return struct{}{}, fmt.Errorf("%w: cannot connect to the D-Bus session bus: %v", ErrUnavailable, err)
		}
		defer conn.Close()

		// GetNameOwner detects a *running* provider without triggering
		// D-Bus service activation (which could start an agent and prompt).
		var owner string
		err = conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, secretServiceBusName).Store(&owner)
		if err != nil || owner == "" {
			return struct{}{}, fmt.Errorf("%w: no Secret Service provider", ErrUnavailable)
		}

		// Reading the Locked property is side-effect free; anything but an
		// unlocked default collection means using the store would prompt.
		locked, err := conn.Object(secretServiceBusName, defaultCollectionPath).GetProperty(collectionLockedProperty)
		if err != nil {
			return struct{}{}, fmt.Errorf("%w: cannot read the default secret collection: %v", ErrUnavailable, err)
		}
		isLocked, ok := locked.Value().(bool)
		if !ok {
			return struct{}{}, fmt.Errorf("%w: cannot read the default secret collection: "+
				"unexpected Locked property type %T", ErrUnavailable, locked.Value())
		}
		if isLocked {
			return struct{}{}, fmt.Errorf(
				"%w: secret collection is locked; unlocking would require a prompt", ErrUnavailable)
		}
		return struct{}{}, nil
	})
	return err
}
