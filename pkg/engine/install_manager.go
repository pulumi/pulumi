// Copyright 2026, Pulumi Corporation.
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

package engine

import (
	"golang.org/x/sync/errgroup"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

// installManager manages concurrent installation of plugins and policy packs.
type installManager struct {
	tasks              errgroup.Group
	returnPluginErrors bool
}

// newInstallManager creates a new installManager.
func newInstallManager(returnPluginErrors bool) *installManager {
	return &installManager{
		returnPluginErrors: returnPluginErrors,
	}
}

// InstallPlugin schedules a plugin installation. If returnPluginErrors is
// false, installation errors will be logged but not returned.
func (im *installManager) InstallPlugin(f func() error) {
	if im.returnPluginErrors {
		im.tasks.Go(f)
	} else {
		im.tasks.Go(func() error {
			if err := f(); err != nil {
				logging.V(7).Infof("InstallPlugin(): failed to install plugin: %v", err)
			}
			return nil
		})
	}
}

// InstallPolicyPack schedules a policy pack installation.
func (im *installManager) InstallPolicyPack(f func() error) {
	im.tasks.Go(f)
}

// Wait waits for all scheduled installations to complete, returning any errors.
func (im *installManager) Wait() error {
	return im.tasks.Wait()
}
