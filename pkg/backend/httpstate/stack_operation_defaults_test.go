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

package httpstate

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/stretchr/testify/require"
)

func TestStackOperationDefaults(t *testing.T) {
	t.Parallel()
	enabled := apitype.StackOperationDefaults{Refresh: new(true), ContinueOnError: new(true), RunProgram: new(true)}
	disabled := apitype.StackOperationDefaults{Refresh: new(false), ContinueOnError: new(false), RunProgram: new(false)}
	for _, tt := range []struct {
		name          string
		kind          apitype.UpdateKind
		defaults      apitype.StackOperationDefaults
		local         *apitype.StackOperationDefaults
		initial, want engine.UpdateOptions
	}{
		{
			name: "update", kind: apitype.UpdateUpdate, defaults: enabled, local: &apitype.StackOperationDefaults{},
			want: engine.UpdateOptions{Refresh: true, RefreshProgram: true, ContinueOnError: true},
		},
		{
			name: "preview", kind: apitype.PreviewUpdate, defaults: enabled, local: &apitype.StackOperationDefaults{},
			want: engine.UpdateOptions{Refresh: true, RefreshProgram: true},
		},
		{
			name: "destroy", kind: apitype.DestroyUpdate, defaults: enabled, local: &apitype.StackOperationDefaults{},
			want: engine.UpdateOptions{Refresh: true, DestroyProgram: true, ContinueOnError: true},
		},
		{
			name: "refresh", kind: apitype.RefreshUpdate, defaults: enabled, local: &apitype.StackOperationDefaults{},
			want: engine.UpdateOptions{RefreshProgram: true},
		},
		{name: "explicit false", kind: apitype.UpdateUpdate, defaults: enabled, local: &disabled},
		{
			name: "explicit true", kind: apitype.DestroyUpdate, defaults: disabled, local: &enabled,
			want: engine.UpdateOptions{Refresh: true, DestroyProgram: true, ContinueOnError: true},
		},
		{
			name: "partial override", kind: apitype.UpdateUpdate, defaults: enabled,
			local: &apitype.StackOperationDefaults{Refresh: new(false)},
			want:  engine.UpdateOptions{RefreshProgram: true, ContinueOnError: true},
		},
		{
			name: "unset server fields preserve engine values", kind: apitype.UpdateUpdate,
			local:   &apitype.StackOperationDefaults{},
			initial: engine.UpdateOptions{Refresh: true, RefreshProgram: true, ContinueOnError: true},
			want:    engine.UpdateOptions{Refresh: true, RefreshProgram: true, ContinueOnError: true},
		},
		{
			name: "server false", kind: apitype.DestroyUpdate, defaults: disabled, local: &apitype.StackOperationDefaults{},
			initial: engine.UpdateOptions{Refresh: true, DestroyProgram: true, ContinueOnError: true},
		},
		{name: "backend caller opts out", kind: apitype.UpdateUpdate, defaults: enabled},
		{
			name: "import unaffected", kind: apitype.ResourceImportUpdate,
			defaults: enabled, local: &apitype.StackOperationDefaults{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := backend.UpdateOptions{Engine: tt.initial, StackOperationOverrides: tt.local}
			applyStackOperationDefaults(tt.kind, &opts, tt.defaults)
			require.Equal(t, tt.want, opts.Engine)
		})
	}
}

func TestStackParallelDefaults(t *testing.T) {
	t.Parallel()
	for _, kind := range []apitype.UpdateKind{
		apitype.UpdateUpdate, apitype.PreviewUpdate, apitype.DestroyUpdate, apitype.RefreshUpdate,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			for _, tt := range []struct {
				name          string
				server, local *int
				want          int32
			}{
				{name: "unset", want: 16},
				{name: "server", server: new(2), want: 2},
				{name: "local override", server: new(2), local: new(4), want: 4},
				{name: "explicit unlimited", server: new(2), local: new(0), want: 0},
				{name: "invalid zero", server: new(0), want: 16},
				{name: "invalid negative", server: new(-1), want: 16},
			} {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					opts := backend.UpdateOptions{
						Engine:                  engine.UpdateOptions{Parallel: 16},
						StackOperationOverrides: &apitype.StackOperationDefaults{Parallel: tt.local},
					}
					applyStackOperationDefaults(kind, &opts, apitype.StackOperationDefaults{Parallel: tt.server})
					require.Equal(t, tt.want, opts.Engine.Parallel)
				})
			}
		})
	}
	for _, kind := range []apitype.UpdateKind{
		apitype.ResourceImportUpdate, apitype.StackImportUpdate, apitype.RenameUpdate,
	} {
		opts := backend.UpdateOptions{
			Engine:                  engine.UpdateOptions{Parallel: 16},
			StackOperationOverrides: &apitype.StackOperationDefaults{},
		}
		applyStackOperationDefaults(kind, &opts, apitype.StackOperationDefaults{Parallel: new(2)})
		require.Equal(t, int32(16), opts.Engine.Parallel)
	}
	opts := backend.UpdateOptions{Engine: engine.UpdateOptions{Parallel: 16}}
	applyStackOperationDefaults(apitype.UpdateUpdate, &opts, apitype.StackOperationDefaults{Parallel: new(2)})
	require.Equal(t, int32(16), opts.Engine.Parallel)
}
