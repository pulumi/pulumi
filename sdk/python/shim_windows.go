// Copyright 2020, Pulumi Corporation.
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

package python

import (
	"os"
	"syscall"
)

// This is to trigger a workaround for https://github.com/golang/go/issues/42919
func needsPythonShim(pythonPath string) bool {
	info, err := os.Lstat(pythonPath)
	if err != nil {
		panic(err) // Should never happen!
	}
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return sys.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 &&
			sys.FileAttributes&syscall.FILE_ATTRIBUTE_ARCHIVE != 0
	}
	return false
}
