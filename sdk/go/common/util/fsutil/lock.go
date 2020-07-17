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

package fsutil

import (
	"sync"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"

	"github.com/gofrs/flock"
)

// FileMutex is a mutex that serializes both within and across processes. When acquired, it can be assumed that the
// caller holds exclusive access over te protected resources, even if there are other consumers both within and outside
// of the same process.
type FileMutex struct {
	proclock sync.Mutex   // lock serializing in-process access to the protected resource
	fslock   *flock.Flock // lock serializing out-of-process access to the protected resource
}

// NewFileMutex creates a new FileMutex using the given file as a file lock.
func NewFileMutex(path string) *FileMutex {
	return &FileMutex{
		fslock: flock.New(path),
	}
}

// Lock locks the file mutex. It does this in two phases: first, it locks the process lock, which when held guarantees
// exclusive access to the resource within the current process. Second, with the process lock held, it locks the file
// lock. The flock system call operates on a process granularity and, if one process attempts to lock the same file
// multiple times, flock will consider the lock to be held by the process even if different threads are acquiring the
// lock.
//
// Because of this, the two-pronged approach to locking guarantees exclusive access to the resource by locking a process
// shared mutex and a global shared mutex. Once this method returns without an error, callers can be sure that the
// calling goroutine completely owns the resource.
func (fm *FileMutex) Lock() error {
	fm.proclock.Lock()
	if err := fm.fslock.Lock(); err != nil {
		fm.proclock.Unlock()
		return err
	}

	contract.Assert(fm.fslock.Locked())
	return nil
}

// Unlock unlocks the file mutex. It first unlocks the file lock, which allows other processes to lock the file lock,
// after which it unlocks the proc lock. Unlocking the file lock first ensures that it is not possible for two
// goroutines to lock or unlock the file mutex without first holding the proc lock.
func (fm *FileMutex) Unlock() error {
	if err := fm.fslock.Unlock(); err != nil {
		fm.proclock.Unlock()
		return err
	}

	fm.proclock.Unlock()
	return nil
}
