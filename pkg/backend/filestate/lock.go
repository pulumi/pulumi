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

package filestate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

type lockContent struct {
	Pid       int       `json:"pid"`
	Username  string    `json:"username"`
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
}

func newLockContent() (*lockContent, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	return &lockContent{
		Pid:       os.Getpid(),
		Username:  u.Username,
		Hostname:  hostname,
		Timestamp: time.Now(),
	}, nil
}

func (l *lockContent) String() string {
	return fmt.Sprintf("%v@%v (pid %v) at %v", l.Username, l.Hostname, l.Pid, l.Timestamp.Format(time.RFC3339))
}

func (b *localBackend) checkForLock(ctxt context.Context, stackRef backend.StackReference) error {
	allFiles, err := listBucket(b.bucket, b.lockDir())
	if err != nil {
		return err
	}

	var lockKeys []string
	for i := len(allFiles) - 1; i >= 0; i-- {
		if allFiles[i].IsDir {
			continue
		}
		fileKey := allFiles[i].Key
		if b.isLockForThisStack(fileKey, stackRef) {
			lockKeys = append(lockKeys, fileKey)
		}
	}

	if len(lockKeys) > 0 {
		errorString := fmt.Sprintf("the stack is current locked by %v lock(s). Either wait for the other "+
			"process(es) to end or manually delete the lock file(s).", len(lockKeys))

		for _, lock := range lockKeys {
			content, err := b.bucket.ReadAll(ctxt, lock)
			if err != nil {
				return err
			}
			l := &lockContent{}
			err = json.Unmarshal(content, &l)
			if err != nil {
				return err
			}

			// this is kind of weird but necessary because url is a string and not a url.URL
			url := b.url + filepath.Join("/", lock)
			errorString = errorString + fmt.Sprintf("\n  %v: created by %v", url, l.String())
		}

		return fmt.Errorf(errorString)
	}
	return nil
}

func (b *localBackend) checkForLockRace(stackRef backend.StackReference) error {
	// Check the locks to make sure ONLY our lock exists. If we find multiple locks then we know
	// we had a race condition and we should clean up and abort
	allFiles, err := listBucket(b.bucket, b.lockDir())
	if err != nil {
		return err
	}

	for i := len(allFiles) - 1; i >= 0; i-- {
		file := allFiles[i].Key
		if b.isLockForThisStack(file, stackRef) && file != b.lockPath(stackRef.Name()) {
			return fmt.Errorf("another Pulumi execution grabbed the lock, please wait" +
				"for the other Pulumi process to complete and try again")
		}
	}
	return nil
}

func (b *localBackend) Lock(stackRef backend.StackReference) error {
	fmt.Printf("Locking %s\n", stackRef.String())
	ctxt := context.TODO()

	err := b.checkForLock(ctxt, stackRef)
	if err != nil {
		return err
	}

	lockContent, err := newLockContent()
	if err != nil {
		return err
	}

	content, err := json.Marshal(lockContent)
	if err != nil {
		return err
	}

	err = b.bucket.WriteAll(ctxt, b.lockPath(stackRef.Name()), content, nil)
	if err != nil {
		return err
	}

	err = b.checkForLockRace(stackRef)
	if err != nil {
		b.Unlock(stackRef)
		return err
	}

	return nil
}

func (b *localBackend) Unlock(stackRef backend.StackReference) {
	err := b.bucket.Delete(context.TODO(), b.lockPath(stackRef.Name()))
	if err != nil {
		logging.Errorf("there was a problem deleting the lock at %v, things may have been left in a bad "+
			"state and a manual clean up may be required: %v",
			filepath.Join(b.url, b.lockPath(stackRef.Name())), err)
	}
}

func (b *localBackend) isLockForThisStack(file string, stackRef backend.StackReference) bool {
	return strings.HasPrefix(file, b.lockPrefix(stackRef.Name()))
}

func (b *localBackend) lockDir() string {
	return filepath.Join(workspace.BookkeepingDir, workspace.LockDir)
}

func (b *localBackend) lockPrefix(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return filepath.Join(b.lockDir(), fsutil.QnamePath(stack)+".")
}

func (b *localBackend) lockPath(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return b.lockPrefix(stack) + b.lockID + ".json"
}
