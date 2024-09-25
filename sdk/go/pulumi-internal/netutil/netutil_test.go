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
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReturnsPortIfOpen(t *testing.T) {
	t.Parallel()

	// "random" port for testing
	port := 57134
	if !isPortAvailable(port) {
		t.Skip("port 57134 is not available")
	}
	p, err := FindNextAvailablePort(port)
	require.NoError(t, err)
	require.Equal(t, port, p)
}

func TestReturnsErrorIfPortNumberTooHigh(t *testing.T) {
	t.Parallel()

	port := 65535
	_, err := FindNextAvailablePort(port)
	require.ErrorContains(t, err, "no open ports found")
}

func TestReturnsNextPortIfNotAvailable(t *testing.T) {
	t.Parallel()

	// "random" port for testing
	port := 58943
	if !isPortAvailable(port + 1) {
		t.Skip("port 58944 is not available")
	}
	// Open a listener on the port to make it unavailable.
	// Ignore the error.  If the port is already open that's also fine.
	l, _ := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	defer l.Close()
	availablePort, err := FindNextAvailablePort(port)
	require.NoError(t, err)
	require.Equal(t, port+1, availablePort)
}
