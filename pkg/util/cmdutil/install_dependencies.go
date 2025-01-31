// Copyright 2024, Pulumi Corporation.
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

package cmdutil

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
)

// InstallDependencies installs dependencies for the given language runtime, blocking until the installation is
// complete. Standard output and error are streamed to os.Stdout and os.Stderr, respectively, and any errors encountered
// during the installation or streaming of its output are returned.
func InstallDependencies(lang plugin.LanguageRuntime, req plugin.InstallDependenciesRequest) error {
	stdout, stderr, done, err := lang.InstallDependencies(req)
	if err != nil {
		return err
	}

	// We'll use a WaitGroup to wait for the stdout (1) and stderr (2) readers to be fully drained, as well as for the
	// done channel to close (3), before we return.
	var wg sync.WaitGroup
	wg.Add(3)

	errorChan := make(chan error, 3)

	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stdout, stdout); err != nil {
			errorChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(os.Stderr, stderr); err != nil {
			errorChan <- err
		}
	}()

	go func() {
		defer wg.Done()
		if err := <-done; err != nil {
			errorChan <- err
		}
	}()

	var errs []error
	wg.Wait()
	close(errorChan)
	for err := range errorChan {
		if err != nil {
			errs = append(errs, err)
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return err
	}

	return nil
}
