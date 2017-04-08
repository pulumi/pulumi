// Copyright 2017 Pulumi, Inc. All rights reserved.

package resource

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"google.golang.org/grpc"

	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/util/contract"
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
				return nil, err
			}
		}
	}

	// If we didn't find anything, we're done.
	if plug == nil {
		contract.Assert(err != nil)
		return nil, err
	}

	// Now that we have a process, we expect it to write a single line to STDOUT: the port it's listening on.  We only
	// read a byte at a time so that STDOUT contains everything after the first newline.
	var port string
	b := make([]byte, 1)
	for {
		n, err := plug.Stdout.Read(b)
		if err != nil {
			plug.Proc.Kill()
			return nil, err
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		port += string(b[:n])
	}

	// Parse the output line (minus the '\n') to ensure it's a numeric port.
	if _, err = strconv.Atoi(port); err != nil {
		plug.Proc.Kill()
		return nil, fmt.Errorf("%v plugin '%v' wrote a non-numeric port to stdout ('%v'): %v",
			prefix, foundbin, port, err)
	}

	// For now, we will spawn goroutines that will spew STDOUT/STDERR to the relevent diag streams.
	// TODO: eventually we want real progress reporting, etc., which will need to be done out of band via RPC.  This
	//     will be particularly important when we parallelize the application of the resource graph.
	tracers := []struct {
		r   io.Reader
		lbl string
		cb  func(string)
	}{
		{plug.Stdout, "stdout", func(line string) { ctx.Diag.Infof(diag.Message(line)) }},
		{plug.Stderr, "stderr", func(line string) { ctx.Diag.Errorf(diag.Message(line)) }},
	}
	for _, trace := range tracers {
		t := trace
		reader := bufio.NewReader(t.r)
		go func() {
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				t.cb(fmt.Sprintf("%v.%v: %v", prefix, t.lbl, line[:len(line)-1]))
			}
		}()
	}

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial(":"+port, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	plug.Conn = conn
	return plug, nil
}

func execPlugin(bin string) (*plugin, error) {
	cmd := exec.Command(bin)
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
