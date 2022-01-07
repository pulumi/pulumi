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
	"io/ioutil"
	"log"
	"os"
	"os/exec"
)

func main() {
	depth := flag.Int("depth", 0, "depth of the process tree")
	out := flag.String("out", "", "path to touch at the bottom of the process tree")

	handle(flag.CommandLine.Parse(os.Args[1:]))

	if *depth > 0 {
		cmd := exec.Command("go",
			"run",
			"processtree.go",
			"--depth", fmt.Sprintf("%d", *depth-1),
			"--out", *out)
		handle(cmd.Start())
		handle(cmd.Process.Release())
	}

	if *depth == 0 {
		handle(ioutil.WriteFile(*out, []byte("OK"), 0600))
	}
}

func handle(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
