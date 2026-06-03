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

package cli

import (
	"context"
	"io"

	esc_pager "github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/pager"
)

type pager interface {
	Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error
}

type defaultPager int

func newPager() pager {
	return defaultPager(0)
}

func (defaultPager) Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	return esc_pager.Run(pager, stdout, stderr, f)
}
