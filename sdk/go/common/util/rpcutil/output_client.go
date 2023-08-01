// Copyright 2016-2022, Pulumi Corporation.
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

package rpcutil

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/hashicorp/go-multierror"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ptyCloser struct {
	done     chan (error)
	pty, tty *os.File
}

func (w *ptyCloser) Close() error {
	// Close can be called multiple times, but we will of nil'd out everything first time.
	if w.done == nil {
		contract.Assertf(w.pty == nil, "ptyCloser.Close called twice: pty is nil")
		contract.Assertf(w.tty == nil, "ptyCloser.Close called twice: tty is nil")
		return nil
	}

	// Try to close the tty
	terr := w.tty.Close()
	// Wait for the done signal
	err := <-w.done
	// Now close the pty
	perr := w.pty.Close()

	// if err is an error because pty closed ignore it
	if ioErr, ok := err.(*fs.PathError); ok {
		if sysErr, ok := ioErr.Err.(syscall.Errno); ok {
			if sysErr == syscall.EIO {
				err = nil
			}
		}
	}

	w.done = nil
	w.pty = nil
	w.tty = nil

	return multierror.Append(err, terr, perr).ErrorOrNil()
}

type nopCloser struct{}

func (w *nopCloser) Close() error { return nil }

type clientWriter struct {
	client  pulumirpc.OutputClient
	isError bool
}

func (w *clientWriter) Write(p []byte) (int, error) {
	var request pulumirpc.WriteRequest
	if w.isError {
		request.Data = &pulumirpc.WriteRequest_Stderr{Stderr: p}
	} else {
		request.Data = &pulumirpc.WriteRequest_Stdout{Stdout: p}
	}

	_, err := w.client.Write(context.Background(), &request)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func DialOutputClient(ctx context.Context, target string) (pulumirpc.OutputClient, error) {
	conn, err := grpc.Dial(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		GrpcChannelOptions(),
	)
	if err != nil {
		return nil, err
	}

	return pulumirpc.NewOutputClient(conn), nil
}

func BindOutputClient(ctx context.Context, client pulumirpc.OutputClient) (io.Closer, io.Writer, io.Writer, error) {
	contract.Requiref(client != nil, "client", "must not be nil")

	// Check capabilities to see if we should spawn a pty for this client
	capabilities, err := client.GetCapabilities(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get capabilities: %w", err)
	}

	stdout := &clientWriter{client: client, isError: false}
	stderr := &clientWriter{client: client, isError: true}

	var pt, tt *os.File
	if capabilities.IsTerminal {
		logging.V(11).Infoln("Opening pseudo terminal")
		pt, tt, err = openPty()
		if err == errUnsupported {
			logging.V(11).Infoln("Pseudo terminal not supported")
			// Fall through, just return plain stdout/err pipes
		} else if err != nil {
			// Fall through, just return plain stdout/err pipes but warn that we tried and failed to make a
			// pty (with coloring because isTerminal means the other side understands ANSI codes)
			_, err := stderr.Write([]byte(colors.Always.Colorize(
				colors.SpecWarning + "warning: could not open pty: " + err.Error() + colors.Reset + "\n")))
			// We couldn't write to stderr, just fail somethings gone very wrong
			if err != nil {
				return nil, nil, nil, fmt.Errorf("write to stderr: %w", err)
			}
		}
	}

	if tt != nil {
		// tt is not nil, so we need to return it _directly_ so that later code can see this is an io.File.
		ptyDone := make(chan error, 1)
		ptyCloser := &ptyCloser{
			pty:  pt,
			tty:  tt,
			done: ptyDone,
		}

		go func() {
			_, err = io.Copy(stdout, pt)
			ptyDone <- err
		}()

		return ptyCloser, tt, tt, nil
	}

	return &nopCloser{}, stdout, stderr, nil
}
