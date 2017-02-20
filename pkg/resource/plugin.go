// Copyright 2016 Marapongo, Inc. All rights reserved.

package resource

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"github.com/marapongo/mu/pkg/tokens"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
	"github.com/marapongo/mu/sdk/go/pkg/murpc"
)

const pluginPrefix = "mu-ressrv"

// Plugin reflects a resource plugin, loaded dynamically for a single package.
type Plugin struct {
	ctx    *Context
	pkg    tokens.Package
	proc   *os.Process
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	conn   *grpc.ClientConn
	client murpc.ResourceProviderClient
}

// NewPlugin attempts to bind to a given package's resource plugin and then creates a gRPC connection to it.  If the
// plugin could not be found, or an error occurs while creating the child process, an error is returned.
func NewPlugin(ctx *Context, pkg tokens.Package) (*Plugin, error) {
	var proc *os.Process
	var procin io.WriteCloser
	var procout io.ReadCloser
	var procerr io.ReadCloser

	// To load a plugin, we first attempt using a well-known name "mu-ressrv-<pkg>".  Note that because <pkg> is a
	// qualified name, it could contain "/" characters which would obviously cause problems; so we substitute "_"s.
	// TODO: on Windows, I suppose we will need to append a ".EXE".
	var err error
	srvexe := pluginPrefix + "-" + strings.Replace(string(pkg), tokens.QNameDelimiter, "_", -1)
	if proc, procin, procout, procerr, err = execPlugin(srvexe); err != nil {
		// If this fails, we will explicitly look in the workspace library, to see if this library has been installed.
		if execerr, isexecerr := err.(*exec.Error); isexecerr && execerr.Err == exec.ErrNotFound {
			libexe := filepath.Join(workspace.InstallRoot(), workspace.InstallRootLibdir, string(pkg), srvexe)
			if proc, procin, procout, procerr, err = execPlugin(libexe); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Now that we have a process, we expect it to write a single line to STDOUT: the port it's listening on.  We only
	// read a byte at a time so that STDOUT contains everything after the first newline.
	var port string
	b := make([]byte, 1)
	for {
		n, err := procout.Read(b)
		if err != nil {
			proc.Kill()
			return nil, err
		}
		if n > 0 && b[0] == '\n' {
			break
		}
		port += string(b[:n])
	}

	// Parse the output line (minus the '\n') to ensure it's a numeric port.
	if _, err = strconv.Atoi(port); err != nil {
		proc.Kill()
		return nil, errors.New(
			fmt.Sprintf("resource provider plugin '%v' wrote a non-numeric port to stdout ('%v'): %v",
				pkg, port, err))
	}

	// Now that we have the port, go ahead and create a gRPC client connection to it.
	conn, err := grpc.Dial(":"+port, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	// TODO: consider some diagnostics that monitor STDOUT/STDERR.
	return &Plugin{
		pkg:    pkg,
		proc:   proc,
		stdin:  procin,
		stdout: procout,
		stderr: procerr,
		conn:   conn,
		client: murpc.NewResourceProviderClient(conn),
	}, nil
}

func execPlugin(name string) (*os.Process, io.WriteCloser, io.ReadCloser, io.ReadCloser, error) {
	cmd := exec.Command(name)
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	err, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, nil, err
	}
	return cmd.Process, in, out, err, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.
func (p *Plugin) Create(res Resource) (ID, error, ResourceState) {
	t := string(res.Type())
	req := &murpc.CreateRequest{
		Type:       t,
		Properties: MarshalProperties(res.Properties()),
	}

	resp, err := p.client.Create(p.ctx.Request(), req)
	if err != nil {
		return ID(""), err, StateUnknown
	}

	id := ID(resp.GetId())
	if id == "" {
		return id,
			errors.New(fmt.Sprintf("plugin for package '%v' returned empty ID from create '%v'", p.pkg, t)),
			StateUnknown
	}
	return id, nil, StateOK
}

// Read reads the instance state identified by id/t, and returns a bag of properties.
func (p *Plugin) Read(id ID, t tokens.Type) (PropertyMap, error) {
	req := &murpc.ReadRequest{
		Id:   string(id),
		Type: string(t),
	}

	resp, err := p.client.Read(p.ctx.Request(), req)
	if err != nil {
		return nil, err
	}

	return UnmarshalProperties(resp.GetProperties()), nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *Plugin) Update(old Resource, new Resource) (ID, error, ResourceState) {
	contract.Requiref(old.ID() != "", "old.ID", "not empty")
	contract.Requiref(new.ID() != "", "old.ID", "not empty")
	contract.Requiref(old.ID() == new.ID(), "old.ID, new.ID", "==")
	contract.Requiref(old.Type() != "", "old.Type", "not empty")
	contract.Requiref(new.Type() != "", "new.Type", "not empty")
	contract.Requiref(old.Type() == new.Type(), "old.Type, new.Type", "==")

	req := &murpc.UpdateRequest{
		Id:   string(old.ID()),
		Type: string(old.Type()),
		Olds: MarshalProperties(old.Properties()),
		News: MarshalProperties(new.Properties()),
	}

	resp, err := p.client.Update(p.ctx.Request(), req)
	if err != nil {
		return ID(""), err, StateUnknown
	}

	return ID(resp.GetId()), nil, StateOK
}

// Delete tears down an existing resource.
func (p *Plugin) Delete(res Resource) (error, ResourceState) {
	contract.Requiref(res.ID() != "", "res.ID", "not empty")
	contract.Requiref(res.Type() != "", "res.Type", "not empty")

	req := &murpc.DeleteRequest{
		Id:   string(res.ID()),
		Type: string(res.Type()),
	}

	if _, err := p.client.Delete(p.ctx.Request(), req); err != nil {
		return err, StateUnknown
	}

	return nil, StateOK
}

// Close tears down the underlying plugin RPC connection and process.
func (p *Plugin) Close() error {
	cerr := p.conn.Close()
	// TODO: consider a more graceful termination than just SIGKILL.
	if err := p.proc.Kill(); err != nil {
		return err
	}
	return cerr
}
