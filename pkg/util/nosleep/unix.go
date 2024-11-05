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
	"github.com/godbus/dbus/v5"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

func keepRunning() DoneFunc {
	conn, err := dbus.SessionBus()
	if err != nil {
		return func() {}
	}

	applicationName := "pulumi"
	reasonForInhibit := "stay awake"
	var cookie uint32
	// Use the gnome power manager.  Wakepy also supports org.freedesktop.PowerManagement, but that seems to be deprecated.
	// Docs for this interface can be found at
	// https://lira.no-ip.org:8443/doc/gnome-session/dbus/gnome-session.html#org.gnome.SessionManager.Inhibit
	obj := conn.Object("org.gnome.SessionManager", "/org/gnome/SessionManager")
	inhibitIdleFlag := uint(8) // inhibit the session from being marked as idle
	toplevelXid := uint(42)    // value doesn't seem to matter here
	err = obj.Call(
		"org.gnome.SessionManager.Inhibit",
		0,
		applicationName,
		toplevelXid,
		reasonForInhibit,
		inhibitIdleFlag).Store(&cookie)
	if err != nil {
		logging.V(5).Infof("Failed to get wake lock: %v", err)
		// We did not succeed in setting up the inhibit, so just return a no-op function
		return func() {}
	}
	logging.V(5).Infof("Got wake lock (gnome session manager, with cookie %d)", cookie)
	return func() {
		_ = obj.Call("org.gnome.SessionManager.Uninhibit", 0, cookie).Store()
		logging.V(5).Infof("Released wake lock (gnome session manager, with cookie %d)", cookie)
	}
}
