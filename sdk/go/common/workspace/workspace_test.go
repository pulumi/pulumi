// Copyright 2016-2021, Pulumi Corporation.
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

package workspace

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWorkspaceReadWriteUnderContention(t *testing.T) {
	n := 100

	wg := &sync.WaitGroup{}
	wg.Add(n)

	var error error

	for j := 0; j < n; j++ {
		go func(i int) {
			defer wg.Done()
			time.Sleep(1 * time.Millisecond)

			w, err := NewFrom("../../auto/test/testproj")
			if err != nil {
				error = err
				return
			}

			se := w.Settings()

			if se != nil {
				se.Stack = strings.Repeat(fmt.Sprintf("abra-cadabra-i-%d", i), 1024*16)
				err := w.Save()
				if err != nil {
					error = err
					return
				}
			}
		}(j)
	}

	wg.Wait()
	assert.NoError(t, error)
}
