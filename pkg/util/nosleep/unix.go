// Copyright 2024, Pulumi Corporation.
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

//go:build freebsd || linux || netbsd || openbsd || solaris || dragonfly
// +build freebsd linux netbsd openbsd solaris dragonfly

package nosleep

import (
	"errors"

	"github.com/godbus/dbus/v5"
)

func keepRunning() DoneFunc {
	conn, err := dbus.SessionBus()
	if err != nil {
		return func() {}
	}

	var cookie uint32
	obj := conn.Object("org.freedesktop.PowerManagement", "/org/freedesktop/PowerManagement/Inhibit")
	err = obj.Call("Inhibit", 0, "nosleep", "stay awake").Store(&cookie)
	err = conn.BusObject().Call("org.freedesktop.PowerManagement", 0, "nosleep", "Keep the system awake").Store()
	var dbusErr dbus.Error
	if errors.As(err, &dbusErr) {
		if dbusErr.Name == "org.freedesktop.DBus.Error.UnknownInterface" {
			// The org.freedesktop.PowerManagement interface is not available, try the Gnome interface
			obj = conn.Object("org.gnome.SessionManager", "/org/gnome/SessionManager")
			err = obj.Call("Inhibit", 0, "nosleep", "stay awake").Store(&cookie)
			if err != nil {
				// We did not succeed in setting up the inhibit, so just return a no-op function
				return func() {}
			}
			return func() {
				obj.Call("UnInhibit", 0, cookie).Store()
			}
		}
	}
	return func() {
		obj.Call("UnInhibit", 0, cookie).Store()
	}
}
