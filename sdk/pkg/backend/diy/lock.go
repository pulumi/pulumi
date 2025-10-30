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

package diy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
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

// checkForLock looks for any existing locks for this stack, and returns a helpful diagnostic if there is one.
func (b *diyBackend) checkForLock(ctx context.Context, stackRef backend.StackReference) error {
	stackName := stackRef.FullyQualifiedName()
	allFiles, err := listBucket(ctx, b.bucket, stackLockDir(stackName))
	if err != nil {
		return err
	}

	// lockPath may return a path with backslashes (\) on Windows.
	// We need to convert it to a slash path (/) to compare it to
	// the keys in the bucket which are always slash paths.
	wantLock := filepath.ToSlash(b.lockPath(stackRef))
	var lockKeys []string
	for _, file := range allFiles {
		if file.IsDir {
			continue
		}
		if file.Key != wantLock {
			lockKeys = append(lockKeys, file.Key)
		}
	}

	if len(lockKeys) > 0 {
		errorString := fmt.Sprintf("the stack is currently locked by %v lock(s). Either wait for the other "+
			"process(es) to end or delete the lock file with `pulumi cancel`.", len(lockKeys))

		for _, lock := range lockKeys {
			content, err := b.bucket.ReadAll(ctx, lock)
			if err != nil {
				return err
			}
			l := &lockContent{}
			err = json.Unmarshal(content, &l)
			if err != nil {
				return err
			}

			errorString += fmt.Sprintf("\n  %v: created by %v@%v (pid %v) at %v",
				b.lockURLForError(lock),
				l.Username,
				l.Hostname,
				l.Pid,
				l.Timestamp.Format(time.RFC3339),
			)
		}

		return errors.New(errorString)
	}
	return nil
}

// lockURLForError returns a URL that can be used in error messages to help users find the lock file.
func (b *diyBackend) lockURLForError(lockPath string) string {
	if parsedURL, err := url.Parse(b.url); err == nil {
		parsedURL.Path = path.Join(parsedURL.Path, lockPath)
		return parsedURL.String()
	}
	// If we couldn't parse the URL, we'll just return a naive concatenation,
	// which is what we used to do before we started using URL parsing.
	return b.url + "/" + lockPath
}

func (b *diyBackend) Lock(ctx context.Context, stackRef backend.StackReference) error {
	//
	err := b.checkForLock(ctx, stackRef)
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
	err = b.bucket.WriteAll(ctx, b.lockPath(stackRef), content, nil)
	if err != nil {
		return err
	}
	err = b.checkForLock(ctx, stackRef)
	if err != nil {
		b.Unlock(ctx, stackRef)
		return err
	}
	return nil
}

func (b *diyBackend) Unlock(ctx context.Context, stackRef backend.StackReference) {
	err := b.bucket.Delete(ctx, b.lockPath(stackRef))
	if err != nil {
		b.d.Errorf(
			diag.Message("", "there was a problem deleting the lock at %v, manual clean up may be required: %v"),
			path.Join(b.url, b.lockPath(stackRef)),
			err)
	}
}

func lockDir() string {
	return path.Join(workspace.BookkeepingDir, workspace.LockDir)
}

func stackLockDir(stack tokens.QName) string {
	contract.Requiref(stack != "", "stack", "must not be empty")
	return path.Join(lockDir(), fsutil.QnamePath(stack))
}

func (b *diyBackend) lockPath(stackRef backend.StackReference) string {
	contract.Requiref(stackRef != nil, "stack", "must not be nil")
	return path.Join(stackLockDir(stackRef.FullyQualifiedName()), b.lockID+".json")
}
