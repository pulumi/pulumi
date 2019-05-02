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
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/fsutil"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/workspace"
	uuid "github.com/satori/go.uuid"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
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

type lockableBackend struct {
	lb     *localBackend
	lockID string
}

func NewLockableBackend(lb *localBackend) Backend {
	return &lockableBackend{lb: lb, lockID: uuid.NewV4().String()}
}

func (b *lockableBackend) CreateStack(ctx context.Context, stackRef backend.StackReference,
	opts interface{}) (backend.Stack, error) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, err
	}
	defer b.Unlock(stackRef)
	return b.lb.CreateStack(ctx, stackRef, opts)
}

func (b *lockableBackend) RemoveStack(ctx context.Context, stackRef backend.StackReference,
	force bool) (bool, error) {
	err := b.Lock(stackRef)
	if err != nil {
		return false, err
	}
	defer b.Unlock(stackRef)
	return b.lb.RemoveStack(ctx, stackRef, force)
}

func (b *lockableBackend) RenameStack(ctx context.Context, stackRef backend.StackReference,
	newName tokens.QName) error {
	err := b.Lock(stackRef)
	if err != nil {
		return err
	}
	defer b.Unlock(stackRef)
	return b.lb.RenameStack(ctx, stackRef, newName)
}

func (b *lockableBackend) Preview(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer b.Unlock(stackRef)
	return b.lb.Preview(ctx, stackRef, op)
}

func (b *lockableBackend) Refresh(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer b.Unlock(stackRef)
	return b.lb.Refresh(ctx, stackRef, op)
}

func (b *lockableBackend) Destroy(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer b.Unlock(stackRef)
	return b.lb.Destroy(ctx, stackRef, op)
}

func (b *lockableBackend) ExportDeployment(ctx context.Context,
	stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, err
	}
	defer b.Unlock(stackRef)
	return b.lb.ExportDeployment(ctx, stackRef)
}

func (b *lockableBackend) ImportDeployment(ctx context.Context, stackRef backend.StackReference,
	deployment *apitype.UntypedDeployment) error {
	err := b.Lock(stackRef)
	if err != nil {
		return err
	}
	defer b.Unlock(stackRef)
	return b.lb.ImportDeployment(ctx, stackRef, deployment)
}

func (b *lockableBackend) Update(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	err := b.Lock(stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	defer b.Unlock(stackRef)
	return b.lb.Update(ctx, stackRef, op)
}

func (b *lockableBackend) UpdateStackTags(ctx context.Context,
	stackRef backend.StackReference, tags map[apitype.StackTagName]string) error {
	err := b.Lock(stackRef)
	if err != nil {
		return err
	}
	defer b.Unlock(stackRef)
	return b.lb.UpdateStackTags(ctx, stackRef, tags)
}

func (b *lockableBackend) checkForLock(ctxt context.Context, stackRef backend.StackReference) error {
	allFiles, err := listBucket(b.lb.bucket, b.lockDir())
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
			content, err := b.lb.bucket.ReadAll(ctxt, lock)
			if err != nil {
				return err
			}
			l := &lockContent{}
			err = json.Unmarshal(content, &l)
			if err != nil {
				return err
			}

			// this is kind of weird but necessary because url is a string and not a url.URL
			url := b.lb.url + filepath.Join("/", lock)
			errorString = errorString + fmt.Sprintf("\n  %v: created by %v", url, l.String())
		}

		return fmt.Errorf(errorString)
	}
	return nil
}

func (b *lockableBackend) checkForLockRace(stackRef backend.StackReference) error {
	// Check the locks to make sure ONLY our lock exists. If we find multiple locks then we know
	// we had a race condition and we should clean up and abort
	allFiles, err := listBucket(b.lb.bucket, b.lockDir())
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

func (b *lockableBackend) Lock(stackRef backend.StackReference) error {
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

	err = b.lb.bucket.WriteAll(ctxt, b.lockPath(stackRef.Name()), content, nil)
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

func (b *lockableBackend) Unlock(stackRef backend.StackReference) {
	err := b.lb.bucket.Delete(context.TODO(), b.lockPath(stackRef.Name()))
	if err != nil {
		logging.Errorf("there was a problem deleting the lock at %v, things may have been left in a bad "+
			"state and a manual clean up may be required: %v",
			filepath.Join(b.lb.url, b.lockPath(stackRef.Name())), err)
	}
}

func (b *lockableBackend) isLockForThisStack(file string, stackRef backend.StackReference) bool {
	return strings.HasPrefix(file, b.lockPrefix(stackRef.Name()))
}

func (b *lockableBackend) lockDir() string {
	return filepath.Join(workspace.BookkeepingDir, workspace.LockDir)
}

func (b *lockableBackend) lockPrefix(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return filepath.Join(b.lockDir(), fsutil.QnamePath(stack)+".")
}

func (b *lockableBackend) lockPath(stack tokens.QName) string {
	contract.Require(stack != "", "stack")
	return b.lockPrefix(stack) + b.lockID + ".json"
}

func (b *lockableBackend) ListStacks(ctx context.Context,
	projectFilter *tokens.PackageName) ([]backend.StackSummary, error) {
	return b.lb.ListStacks(ctx, projectFilter)
}
func (b *lockableBackend) Query(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) result.Result {
	return b.lb.Query(ctx, stackRef, op)
}

func (b *lockableBackend) GetHistory(ctx context.Context,
	stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	return b.lb.GetHistory(ctx, stackRef)
}

func (b *lockableBackend) GetLogs(ctx context.Context, stackRef backend.StackReference,
	cfg backend.StackConfiguration,
	query operations.LogQuery) ([]operations.LogEntry, error) {
	return b.lb.GetLogs(ctx, stackRef, cfg, query)
}

func (b *lockableBackend) GetLatestConfiguration(ctx context.Context,
	stackRef backend.StackReference) (config.Map, error) {
	return b.lb.GetLatestConfiguration(ctx, stackRef)
}

func (b *lockableBackend) GetStackTags(ctx context.Context,
	stackRef backend.StackReference) (map[apitype.StackTagName]string, error) {
	return b.lb.GetStackTags(ctx, stackRef)
}
func (b *lockableBackend) Logout() error {
	return b.lb.Logout()
}
func (b *lockableBackend) CurrentUser() (string, error) {
	return b.lb.CurrentUser()
}

func (b *lockableBackend) Name() string {
	return b.lb.Name()
}

func (b *lockableBackend) URL() string {
	return b.lb.URL()
}

func (b *lockableBackend) ParseStackReference(stackRefName string) (backend.StackReference, error) {
	return b.lb.ParseStackReference(stackRefName)
}

func (b *lockableBackend) GetStack(ctx context.Context,
	stackRef backend.StackReference) (backend.Stack, error) {
	return b.lb.GetStack(ctx, stackRef)
}

func (b *lockableBackend) local() {
}
