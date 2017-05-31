// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/pulumi/lumi/pkg/diag"
	"github.com/pulumi/lumi/pkg/util/cmdutil"
	"github.com/pulumi/lumi/pkg/util/contract"
)

type plugin struct {
	Conn   *grpc.ClientConn
	Proc   *os.Process
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func newPlugin(ctx *Context, bins []string, prefix string) (*plugin, error) {
	// Try all of the search paths given in the bin argument, either until we find something, or until we've exhausted
	// all available options, whichever comes first.
	var plug *plugin
	var err error
	var foundbin string
	for _, bin := range bins {
		// Try to execute the binary.
		if plug, err = execPlugin(bin); err == nil {
			// Great!  Break out, we're ready to go.
			foundbin = bin
			break
		} else {
			// If that failed, and it was simply a missing file error, keep searching the paths.
			if execerr, isexecerr := err.(*exec.Error); !isexecerr || execerr.Err != exec.ErrNotFound {
				return nil, errors.Wrapf(err, "plugin [%v] failed to load", bin)
			}
		}
	}

	// If we didn't find anything, we're done.
	if plug == nil {
		contract.Assert(err != nil)
		return nil, err
	}

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevent diag streams.
	// TODO: eventually we want real progress reporting, etc., which will need to be done out of band via RPC.  This
	//     will be particularly important when we parallelize the application of the resource graph.
	tracers := map[io.Reader]struct {
		lbl string
		cb  func(string)
	}{
		plug.Stderr: {"stderr", func(line string) { ctx.Diag.Errorf(diag.Message(line)) }},
		plug.Stdout: {"stdout", func(line string) { ctx.Diag.Infof(diag.Message(line)) }},
	}
	runtrace := func(t io.Reader) {
		ts := tracers[t]
		reader := bufio.NewReader(t)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			ts.cb(fmt.Sprintf("%v.%v: %v", prefix, ts.lbl, line[:len(line)-1]))
		}
	}

	// Set up a tracer on stderr before going any further, since important errors might get communicated this way.
	go runtrace(plug.Stderr)

	// Now that we have a process, we expect it to write a single line to STDOUT: the port it's listening on.  We only
	// read a byte at a time so that STDOUT contains everything after the first newline.
	var port string
	b := make([]byte, 1)
	for {
		n, err := plug.Stdout.Read(b)
		if err != nil {
			plug.Proc.Kill()
			if port == "" {
				return nil, errors.Wrapf(err, "could not read plugin [%v] stdout", foundbin)
			}
			return nil, errors.Wrapf(err, "failure reading plugin [%v] stdout (read '%v')", foundbin, port)
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		port += string(b[:n])
	}

	// Parse the output line (minus the '\n') to ensure it's a numeric port.
	if _, err = strconv.Atoi(port); err != nil {
		plug.Proc.Kill()
		return nil, errors.Wrapf(
			err, "%v plugin [%v] wrote a non-numeric port to stdout ('%v')", prefix, foundbin, port)
	}

	// After reading the port number, set up a tracer on stdout just so other output doesn't disappear.
	go runtrace(plug.Stdout)

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial(":"+port, grpc.WithInsecure())
	if err != nil {
		return nil, errors.Wrapf(err, "could not dial plugin [%v] over RPC", foundbin)
	}
	plug.Conn = conn
	return plug, nil
}

func execPlugin(bin string) (*plugin, error) {
	// Flow the logging information if set.
	var args []string
	if cmdutil.LogFlow {
		if cmdutil.LogToStderr {
			args = append(args, "--logtostderr")
		}
		if cmdutil.Verbose > 0 {
			args = append(args, "-v="+strconv.Itoa(cmdutil.Verbose))
		}
	}

	cmd := exec.Command(bin, args...)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &plugin{
		Proc:   cmd.Process,
		Stdin:  in,
		Stdout: out,
		Stderr: err,
	}, nil
}

func (p *plugin) Close() error {
	closerr := p.Conn.Close()
	// TODO: consider a more graceful termination than just SIGKILL.
	if err := p.Proc.Kill(); err != nil {
		return err
	}
	return closerr
}
