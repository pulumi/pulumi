// Copyright 2024-2024, Pulumi Corporation.
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

package netutil

import (
	"errors"
	"fmt"
	"net"
)

func isPortAvailable(port int) bool {
	if l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
		l.Close()
		return true
	}
	return false
}

func FindNextAvailablePort(startPort int) (int, error) {
	for port := startPort; port < 65535; port++ {
		if isPortAvailable(port) {
			return port, nil
		}
	}
	return 0, errors.New("no open ports found")
}
