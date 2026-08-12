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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	secretServicePath        = dbus.ObjectPath("/org/freedesktop/secrets")
)

// secretServicePrecheck fast-fails before go-keyring ever touches the Secret
// Service: it verifies a session bus exists, that a provider is actually
// running, and that the default collection is unlocked. A locked collection
// gets one unlock attempt, which only draws a dialog when allowPrompt says
// someone is there to answer it.
var secretServicePrecheck = memoizePrecheck(probeSecretService)

func probeSecretService(allowPrompt bool) (Outcome, error) {
	// Cheap environment check first: no session bus address and no
	// user-runtime bus socket means connecting is pointless.
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir == "" {
			return Absent, fmt.Errorf("%w: no D-Bus session bus", ErrUnavailable)
		}
		if _, err := os.Stat(filepath.Join(runtimeDir, "bus")); err != nil {
			return Absent, fmt.Errorf("%w: no D-Bus session bus", ErrUnavailable)
		}
	}

	_, err := withTimeout(keyringOpTimeout, func() (struct{}, error) {
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
			return struct{}{}, fmt.Errorf("%w: no Secret Service provider is running", ErrUnavailable)
		}
		provider := nameOwnerProcess(conn, owner)

		// Reading the Locked property is side-effect free; anything but an
		// unlocked default collection means using the store would prompt.
		locked, err := conn.Object(secretServiceBusName, defaultCollectionPath).GetProperty(collectionLockedProperty)
		if err != nil {
			return struct{}{}, fmt.Errorf("%w: cannot read the default secret collection%s: %v",
				ErrUnavailable, provider, err)
		}
		isLocked, ok := locked.Value().(bool)
		if !ok {
			return struct{}{}, fmt.Errorf("%w: cannot read the default secret collection%s: "+
				"unexpected Locked property type %T", ErrUnavailable, provider, locked.Value())
		}
		if isLocked {
			return struct{}{}, lockedCollectionError{provider: provider}
		}
		return struct{}{}, nil
	})
	var locked lockedCollectionError
	if errors.As(err, &locked) {
		unlockErr := unlockDefaultCollection(allowPrompt)
		switch {
		case unlockErr == nil:
			return Ready, nil
		case errors.Is(unlockErr, ErrDeclined):
			return Declined, unlockErr
		default:
			return Locked, fmt.Errorf("%w%s: %v", ErrLocked, locked.provider, unlockErr)
		}
	}
	if err != nil {
		return Absent, err
	}
	return Ready, nil
}

type lockedCollectionError struct{ provider string }

func (lockedCollectionError) Error() string { return "the default secret collection is locked" }

func unlockDefaultCollection(mayAskUser bool) error {
	if mayAskUser {
		return onSessionBus(unlockAndWaitForUser)
	}
	_, err := withTimeout(keyringOpTimeout, func() (struct{}, error) {
		return struct{}{}, onSessionBus(unlockWithoutDrawingUI)
	})
	return err
}

func onSessionBus(do func(*dbus.Conn) error) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	return do(conn)
}

func unlockWithoutDrawingUI(conn *dbus.Conn) error {
	unlocked, promptPath, err := requestUnlock(conn)
	if err != nil {
		return err
	}
	if len(unlocked) > 0 {
		return nil
	}
	if promptPath != "" && promptPath != "/" {
		_ = conn.Object(secretServiceBusName, promptPath).Call("org.freedesktop.Secret.Prompt.Dismiss", 0).Err
	}
	return errors.New("unlocking the credential store needs a password prompt, and none can be shown here")
}

func confirmUnlocked(conn *dbus.Conn) error {
	locked, err := conn.Object(secretServiceBusName, defaultCollectionPath).GetProperty(collectionLockedProperty)
	if err != nil {
		return err
	}
	if isLocked, ok := locked.Value().(bool); !ok || isLocked {
		return errors.New("the collection is still locked after the unlock attempt")
	}
	return nil
}

func requestUnlock(conn *dbus.Conn) (unlocked []dbus.ObjectPath, prompt dbus.ObjectPath, err error) {
	err = conn.Object(secretServiceBusName, secretServicePath).
		Call("org.freedesktop.Secret.Service.Unlock", 0, []dbus.ObjectPath{defaultCollectionPath}).
		Store(&unlocked, &prompt)
	return unlocked, prompt, err
}

func unlockAndWaitForUser(conn *dbus.Conn) error {
	unlocked, promptPath, err := requestUnlock(conn)
	if err != nil {
		return err
	}
	if len(unlocked) > 0 {
		return nil
	}
	if promptPath == "" || promptPath == "/" {
		return errors.New("unlock returned neither an unlocked collection nor a prompt")
	}

	matchOpts := []dbus.MatchOption{
		dbus.WithMatchObjectPath(promptPath),
		dbus.WithMatchInterface("org.freedesktop.Secret.Prompt"),
		dbus.WithMatchMember("Completed"),
	}
	if err := conn.AddMatchSignal(matchOpts...); err != nil {
		return err
	}
	defer func() { _ = conn.RemoveMatchSignal(matchOpts...) }()
	ownerOpts := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, secretServiceBusName),
	}
	if err := conn.AddMatchSignal(ownerOpts...); err != nil {
		return err
	}
	defer func() { _ = conn.RemoveMatchSignal(ownerOpts...) }()
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	prompt := conn.Object(secretServiceBusName, promptPath)
	// The command ends when the user answers, so the only prompt we ever
	// take down is one we are abandoning ourselves.
	defer func() { _ = prompt.Call("org.freedesktop.Secret.Prompt.Dismiss", 0).Err }()
	shown := showDialogNonBlocking(prompt)
	fmt.Fprintln(os.Stderr,
		"Pulumi needs the key protecting your credentials: answer your keyring's password prompt to continue.")

	for {
		select {
		case call := <-shown:
			// The method reply says only whether the dialog was raised. A
			// failure here means no Completed signal is ever coming, so the
			// wait must end rather than hang on a dialog nobody can see.
			if call.Err != nil {
				return call.Err
			}
			shown = nil
		case sig, ok := <-signals:
			if !ok {
				return errors.New("the session bus closed before the unlock prompt completed")
			}
			switch {
			case sig.Name == "org.freedesktop.DBus.NameOwnerChanged" && providerVanished(sig):
				return errors.New("the credential store stopped while its password prompt was open")
			case sig.Path == promptPath && sig.Name == "org.freedesktop.Secret.Prompt.Completed":
				if len(sig.Body) > 0 {
					if dismissed, isBool := sig.Body[0].(bool); isBool && !dismissed {
						return confirmUnlocked(conn)
					}
				}
				// Providers also report "dismissed" when no prompter could be
				// reached at all, so do not claim the user did it.
				return fmt.Errorf("%w: its password prompt was dismissed or could not be shown", ErrDeclined)
			}
		}
	}
}

func showDialogNonBlocking(prompt dbus.BusObject) <-chan *dbus.Call {
	done := make(chan *dbus.Call, 1)
	prompt.Go("org.freedesktop.Secret.Prompt.Prompt", 0, done, "")
	return done
}

func providerVanished(sig *dbus.Signal) bool {
	if len(sig.Body) < 3 {
		return false
	}
	name, _ := sig.Body[0].(string)
	newOwner, _ := sig.Body[2].(string)
	return name == secretServiceBusName && newOwner == ""
}

func nameOwnerProcess(conn *dbus.Conn, owner string) string {
	var pid uint32
	err := conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, owner).Store(&pid)
	if err != nil {
		return ""
	}
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return ""
	}
	return fmt.Sprintf(" (provider %q)", strings.TrimSpace(string(comm)))
}
