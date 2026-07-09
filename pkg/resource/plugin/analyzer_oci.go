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

package plugin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/rpcutil"
)

// EnvPolicyPackAttach lists policy packs the engine should attach to at a
// known port instead of launching, in the form "<pack-name>:<port>[,…]".
// This is the policy pack analogue of PULUMI_DEBUG_PROVIDERS: it is how packs
// running as pod sidecars are reached, and a debugging affordance for pack
// authors (run the pack under a debugger and point the CLI at it).
const EnvPolicyPackAttach = "PULUMI_POLICY_PACK_ATTACH"

// GetPolicyPackAttachPort returns the attach port for the named policy pack
// from EnvPolicyPackAttach, or nil if the pack is not listed.
func GetPolicyPackAttachPort(name tokens.QName) (*int, error) {
	envVar, has := os.LookupEnv(EnvPolicyPackAttach)
	if !has {
		return nil, nil
	}
	for _, entry := range strings.Split(envVar, ",") {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 || parts[0] != string(name) {
			continue
		}
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("expected a numeric port for %q in %s, got %q: %w",
				parts[0], EnvPolicyPackAttach, parts[1], err)
		}
		return &port, nil
	}
	return nil, nil
}

// dialAnalyzerWithRetry dials addr and waits until the gRPC channel is READY,
// retrying transient failures until timeout. A raw TCP connect is not
// sufficient readiness — container runtimes bind the host port before the pack
// is listening — so we require the channel itself to become READY. If
// containerCheck is non-nil and reports the container has exited, we fail fast
// with its logs instead of waiting out the timeout.
func dialAnalyzerWithRetry(
	ctx context.Context, addr string, timeout time.Duration,
	containerCheck func() (running bool, logs string),
) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		rpcutil.GrpcChannelOptions())
	if err != nil {
		return nil, fmt.Errorf("could not create connection to policy pack at %s: %w", addr, err)
	}

	deadline := time.Now().Add(timeout)
	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}
		if containerCheck != nil {
			if running, logs := containerCheck(); !running {
				contract.IgnoreClose(conn)
				return nil, fmt.Errorf("policy pack container exited before serving its analyzer; container logs:\n%s", logs)
			}
		}
		waitCtx, cancel := context.WithDeadline(ctx, deadline)
		changed := conn.WaitForStateChange(waitCtx, state)
		cancel()
		if !changed {
			var logs string
			if containerCheck != nil {
				_, logs = containerCheck()
				logs = "; container logs:\n" + logs
			}
			contract.IgnoreClose(conn)
			return nil, fmt.Errorf("timed out after %v waiting for policy pack analyzer at %s%s",
				timeout, addr, logs)
		}
	}
}
