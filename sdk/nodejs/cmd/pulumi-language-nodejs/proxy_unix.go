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

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
)

func createPipes() (pipes, error) {
	dir, err := ioutil.TempDir("", "pulumi-node-pipes")
	invokeReqPath, invokeResPath := path.Join(dir, "invoke_req"), path.Join(dir, "invoke_res")

	if err != nil {
		return nil, err
	}
	if err := syscall.Mkfifo(path.Join(dir, "invoke_req"), 0600); err != nil {
		logging.V(10).Infof("createPipes: Received error opening request pipe: %s\n", err)
		return nil, err
	}
	if err := syscall.Mkfifo(path.Join(dir, "invoke_res"), 0600); err != nil {
		logging.V(10).Infof("createPipes: Received error opening result pipe: %s\n", err)
		return nil, err
	}

	invokeReqPipe, err := os.OpenFile(invokeReqPath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	invokeResPipe, err := os.OpenFile(invokeResPath, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}

	return &unixPipes{
		dir:     dir,
		reqPath: invokeReqPath,
		reqPipe: invokeReqPipe,
		resPath: invokeResPath,
		resPipe: invokeResPipe,
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

func (p *unixPipes) shutdown() {
	contract.IgnoreClose(p.reqPipe)
	contract.IgnoreClose(p.resPipe)

	contract.IgnoreError(os.Remove(p.reqPath))
	contract.IgnoreError(os.Remove(p.resPath))

	contract.IgnoreError(os.Remove(p.dir))
}
