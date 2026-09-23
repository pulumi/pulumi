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

package config

import (
	"fmt"
	"strconv"

	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
)

// Operation names the command a stack's environment is opened for.
type Operation string

const (
	OperationPreview Operation = "preview"
	OperationRefresh Operation = "refresh"
	OperationImport  Operation = "import"
	OperationLogs    Operation = "logs"
	OperationConfig  Operation = "config"
	OperationUp      Operation = "up"
	OperationDestroy Operation = "destroy"
	OperationWatch   Operation = "watch"
	OperationDo      Operation = "do"
)

// Privileged reports whether the operation changes resources, and so opens the stack's environment with its
// `includeIn: privileged` imports rather than its `includeIn: unprivileged` ones.
func (op Operation) Privileged() bool {
	return op == OperationUp || op == OperationDestroy || op == OperationWatch || op == OperationDo
}

func stackEnvPrivileged(op Operation) (bool, error) {
	forced := env.ESCPrivileged.Value()
	if forced == "" {
		return op.Privileged(), nil
	}
	privileged, err := strconv.ParseBool(forced)
	if err != nil {
		return false, fmt.Errorf("PULUMI_ESC_PRIVILEGED must be true or false, got %q", forced)
	}
	return privileged, nil
}
