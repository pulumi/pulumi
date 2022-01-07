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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func main() {
	depth := flag.Int("depth", 0, "depth of the process tree")
	handle(flag.CommandLine.Parse(os.Args[1:]))

	if *depth > 0 {
		cmd := exec.Command(os.Args[0], "--depth", fmt.Sprintf("%d", *depth-1))
		handle(cmd.Start())
		handle(cmd.Process.Release())
	}

	for {
		time.Sleep(1 * time.Second)
	}
}

func handle(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
