// Copyright 2016-2018, Pulumi Corporation.
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

//+build !windows

package main

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// Unix specific pipe implementation. Fairly simple as it sits on top of a pair of standard fifo
// files that we can communicate over.  Slightly complex as this involves extra cleanup steps to
// ensure they're cleaned up when we're done.
func createPipes() (pipes, error) {
	dir, err := ioutil.TempDir("", "pulumi-node-pipes")
	if err != nil {
		return nil, err
	}

	invokeReqPath, invokeResPath := path.Join(dir, "invoke_req"), path.Join(dir, "invoke_res")
	return &unixPipes{
		dir:     dir,
		reqPath: invokeReqPath,
		resPath: invokeResPath,
	}, nil
}

type unixPipes struct {
	dir              string
	reqPath, resPath string
	reqPipe, resPipe *os.File
}

func (p *unixPipes) directory() string {
	return p.dir
}

func (p *unixPipes) reader() io.Reader {
	return p.reqPipe
}

func (p *unixPipes) writer() io.Writer {
	return p.resPipe
}

func (p *unixPipes) connect() error {
	if err := syscall.Mkfifo(path.Join(p.dir, "invoke_req"), 0600); err != nil {
		logging.V(10).Infof("createPipes: Received error opening request pipe: %s\n", err)
		return err
	}

	if err := syscall.Mkfifo(path.Join(p.dir, "invoke_res"), 0600); err != nil {
		logging.V(10).Infof("createPipes: Received error opening result pipe: %s\n", err)
		return err
	}

	invokeReqPipe, err := os.OpenFile(p.reqPath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	p.reqPipe = invokeReqPipe

	invokeResPipe, err := os.OpenFile(p.resPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	p.resPipe = invokeResPipe

	return nil
}

func (p *unixPipes) shutdown() {
	if p.reqPipe != nil {
		contract.IgnoreClose(p.reqPipe)
		contract.IgnoreError(os.Remove(p.reqPath))
	}

	if p.resPipe != nil {
		contract.IgnoreClose(p.resPipe)
		contract.IgnoreError(os.Remove(p.resPath))
	}

	contract.IgnoreError(os.Remove(p.dir))
}
