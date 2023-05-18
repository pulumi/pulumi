// Copyright 2016-2023, Pulumi Corporation.
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

package codegentest

import (
	"context"
	"testing"
	"time"

	"regress-go-12971/example"
	"regress-go-12971/example/config"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // modifies environment variables
func TestEnvironmentDefaults(t *testing.T) {
	// This test verifies that properties with default values from
	// environment variables are correctly applied to resources, providers,
	// and configuration.

	checkType := func(t *testing.T, want example.World) {
		got := (&example.World{}).Defaults()
		assert.Equal(t, &example.World{
			Name:      want.Name,
			Populated: want.Populated,
			RadiusKm:  want.RadiusKm,
		}, got)
	}

	checkTypeArgs := func(t *testing.T, want example.World) {
		args := (&example.WorldArgs{}).Defaults()
		got := waitForOutput[example.World](t, args.ToWorldOutput())
		assert.Equal(t, example.World{
			Name:      want.Name,
			Populated: want.Populated,
			RadiusKm:  want.RadiusKm,
		}, got)
	}

	checkConfig := func(t *testing.T, want example.World) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			// For configuration, there's no concept of unset.
			// We use zero values instead.
			// We can modify want because it's passed by value.
			if want.Name == nil {
				want.Name = ptr("")
			}
			if want.Populated == nil {
				want.Populated = ptr(false)
			}
			if want.RadiusKm == nil {
				want.RadiusKm = ptr(0.0)
			}

			assert.Equal(t, *want.Name, config.GetName(ctx), "name")
			assert.Equal(t, *want.Populated, config.GetPopulated(ctx), "populated")
			assert.Equal(t, *want.RadiusKm, config.GetRadiusKm(ctx), "radiusKm")

			return nil
		}, pulumi.WithMocks("project", "stack", &mockResourceMonitor{}))
		require.NoError(t, err)
	}

	checkProvider := func(t *testing.T, want example.World) {
		err := pulumi.RunErr(func(ctx *pulumi.Context) error {
			_, err := example.NewProvider(ctx, "prov", nil)
			return err
		}, pulumi.WithMocks("project", "stack", &mockResourceMonitor{
			NewResourceF: func(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
				require.Equal(t, "pulumi:providers:world", args.TypeToken, "provider type")
				require.Equal(t, "prov", args.Name, "provider name")

				wantMap := make(map[string]any)
				if want.Name != nil {
					wantMap["name"] = *want.Name
				}
				if want.Populated != nil {
					wantMap["populated"] = *want.Populated
				}
				if want.RadiusKm != nil {
					wantMap["radiusKm"] = *want.RadiusKm
				}

				assert.Equal(t, wantMap, args.Inputs.Mappable())
				return args.Name + "_id", args.Inputs, nil
			},
		}))
		require.NoError(t, err)
	}

	tests := []struct {
		desc string
		env  map[string]string
		want example.World
	}{
		{
			desc: "unset",
			want: example.World{ /* all nil */ },
		},
		{
			desc: "string",
			env:  map[string]string{"WORLD_NAME": "Earth"},
			want: example.World{Name: ptr("Earth")},
		},
		{
			desc: "string/empty",
			env:  map[string]string{"WORLD_NAME": ""},
			want: example.World{Name: ptr("")},
		},
		{
			desc: "bool",
			env:  map[string]string{"WORLD_POPULATED": "true"},
			want: example.World{Populated: ptr(true)},
		},
		{
			desc: "bool/false",
			env:  map[string]string{"WORLD_POPULATED": "false"},
			want: example.World{Populated: ptr(false)},
		},
		{
			desc: "number",
			env:  map[string]string{"WORLD_RADIUS_KM": "6378"},
			want: example.World{RadiusKm: ptr(6378.0)},
		},
		{
			desc: "number/zero",
			env:  map[string]string{"WORLD_RADIUS_KM": "0"},
			want: example.World{RadiusKm: ptr(0.0)},
		},
		{
			desc: "all",
			env: map[string]string{
				"WORLD_NAME":      "Earth",
				"WORLD_POPULATED": "true",
				"WORLD_RADIUS_KM": "6378",
			},
			want: example.World{
				Name:      ptr("Earth"),
				Populated: ptr(true),
				RadiusKm:  ptr(6378.0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			t.Run("Config", func(t *testing.T) {
				t.Parallel()

				checkConfig(t, tt.want)
			})

			t.Run("Type/Defaults", func(t *testing.T) {
				t.Parallel()

				checkType(t, tt.want)
			})

			t.Run("TypeArgs/Defaults", func(t *testing.T) {
				t.Parallel()

				checkTypeArgs(t, tt.want)
			})

			t.Run("Provider", func(t *testing.T) {
				t.Parallel()

				checkProvider(t, tt.want)
			})
		})
	}
}

type mockResourceMonitor struct {
	CallF        func(pulumi.MockCallArgs) (resource.PropertyMap, error)
	NewResourceF func(pulumi.MockResourceArgs) (string, resource.PropertyMap, error)
}

func (m *mockResourceMonitor) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	if m.CallF != nil {
		return m.CallF(args)
	}
	return args.Args, nil
}

func (m *mockResourceMonitor) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	if m.NewResourceF != nil {
		return m.NewResourceF(args)
	}
	return args.Name + "_id", args.Inputs, nil
}

// Returns a pointer to the given value.
func ptr[T any](v T) *T { return &v }

// waitForOutut blocks until the given output has resolved.
// The test fails if the output does not resolve after a while.
// This is a bit icky but it suffices for this test.
func waitForOutput[T any](t testing.TB, o pulumi.Output) T {
	done := make(chan struct{})
	var v T

	o.ApplyT(func(x T) T {
		v = x
		close(done)
		return x
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Fatal("timed out waiting for output")
	case <-done:
		// proceed
	}

	return v
}
