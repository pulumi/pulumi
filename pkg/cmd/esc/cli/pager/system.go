// Copyright 2023, Pulumi Corporation.
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

package pager

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
)

func runSystemPager(pager []string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//nolint:gosec
	cmd := exec.Command(pager[0], pager[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("running pager: %w", err)
	}

	done := make(chan error)
	go func() {
		done <- func() error {
			defer stdin.Close()

			w := bufio.NewWriter(stdin)
			defer w.Flush()

			return f(ctx, w)
		}()
	}()

	if cmdErr := cmd.Run(); cmdErr != nil {
		return fmt.Errorf("running pager: %w", cmdErr)
	}
	cancel()

	return <-done
}
