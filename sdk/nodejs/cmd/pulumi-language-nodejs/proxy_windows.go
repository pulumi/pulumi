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

//+build windows

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"time"

	winio "github.com/Microsoft/go-winio"

	"github.com/pulumi/pulumi/pkg/util/logging"
)

func createPipes() (pipes, error) {
	// Generate a random pipe name so that we don't collide with other pipes made by other pulumi
	// instances.
	rand := uint32(time.Now().UnixNano() + int64(os.Getpid()))
	dir := fmt.Sprintf("\\\\.\\pipe\\pulumi%v", rand)
	reqPipeName := path.Join(dir, "invoke_req")
	resPipeName := path.Join(dir, "invoke_res")
	reqListener, err := winio.ListenPipe(reqPipeName, nil)
	if err != nil {
		logging.V(10).Infof("createPipes: Received error trying to create request pipe %s: %s\n", reqPipeName, err)
		return nil, err
	}

	resListener, err := winio.ListenPipe(resPipeName, nil)
	if err != nil {
		logging.V(10).Infof("createPipes: Received error trying to create response pipe %s: %s\n", resPipeName, err)
		return nil, err
	}

	return &windowsPipes{
		dir:         dir,
		reqListener: reqListener,
		resListener: resListener,
	}, nil
}

type windowsPipes struct {
	dir                      string
	reqListener, resListener net.Listener
	reqConn, resConn         net.Conn
}

func (p *windowsPipes) directory() string {
	return p.dir
}

func (p *windowsPipes) reader() io.Reader {
	return p.reqConn
}

func (p *windowsPipes) writer() io.Writer {
	return p.resConn
}

func (p *windowsPipes) connect() error {
	reqConn, err := p.reqListener.Accept()
	if err != nil {
		return err
	}
	p.reqConn = reqConn

	resConn, err := p.resListener.Accept()
	if err != nil {
		return err
	}
	p.resConn = resConn

	return nil
}

func (p *windowsPipes) shutdown() {
	// Don't need to do anything here.  Named pipes are cleaned up when all processes that are using
	// them terminate.
}
