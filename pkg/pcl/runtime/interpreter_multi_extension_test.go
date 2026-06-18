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

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
)

// multiExtensionRuntimeLoader serves two extensions over the same base provider
// ("extbase"): "exta" defines extbase:index:Aye and "extb" defines
// extbase:index:Bee. Both resource tokens live in the base namespace.
type multiExtensionRuntimeLoader struct{}

func (l *multiExtensionRuntimeLoader) LoadPackage(pkg string, version *semver.Version) (*schema.Package, error) {
	return l.LoadPackageV2(context.TODO(), &schema.PackageDescriptor{Name: pkg, Version: version})
}

func (l *multiExtensionRuntimeLoader) LoadPackageV2(
	_ context.Context, d *schema.PackageDescriptor,
) (*schema.Package, error) {
	var name, token string
	switch {
	case d.Parameterization != nil && d.Parameterization.Name == "exta":
		name, token = "exta", "extbase:index:Aye"
	case d.Parameterization != nil && d.Parameterization.Name == "extb":
		name, token = "extb", "extbase:index:Bee"
	default:
		return nil, fmt.Errorf("unexpected package %q", d.PackageName())
	}

	spec := schema.PackageSpec{
		Name:    name,
		Version: "1.0.0",
		Resources: map[string]schema.ResourceSpec{
			token: {ObjectTypeSpec: schema.ObjectTypeSpec{Type: "object"}},
		},
		ExtensionParameterization: &schema.ExtensionParameterizationSpec{
			BaseProvider: schema.BaseProviderRefSpec{Name: "extbase", Version: "45.0.0"},
			Parameter:    []byte(name),
		},
	}
	p, diags, err := schema.BindSpec(spec, nil, schema.ValidationOptions{})
	if err != nil {
		return nil, err
	}
	if diags.HasErrors() {
		return nil, diags
	}
	return p, nil
}

func (l *multiExtensionRuntimeLoader) LoadPackageReference(
	pkg string, version *semver.Version,
) (schema.PackageReference, error) {
	return l.LoadPackageReferenceV2(context.TODO(), &schema.PackageDescriptor{Name: pkg, Version: version})
}

func (l *multiExtensionRuntimeLoader) LoadPackageReferenceV2(
	ctx context.Context, d *schema.PackageDescriptor,
) (schema.PackageReference, error) {
	p, err := l.LoadPackageV2(ctx, d)
	if err != nil {
		return nil, err
	}
	return p.Reference(), nil
}

// registerPackageMonitor returns a distinct package reference per registered
// (extension) package, derived from the parameterization name.
type registerPackageMonitor struct {
	pulumirpc.ResourceMonitorClient
}

func (registerPackageMonitor) RegisterPackage(
	_ context.Context, req *pulumirpc.RegisterPackageRequest, _ ...grpc.CallOption,
) (*pulumirpc.RegisterPackageResponse, error) {
	name := ""
	switch {
	case req.Extension != nil:
		name = req.Extension.Name
	case req.Parameterization != nil:
		name = req.Parameterization.Name
	}
	return &pulumirpc.RegisterPackageResponse{Ref: "ref-" + name}, nil
}

// TestPackageRefResolutionAcrossExtensionsOnSameBase registers two extensions
// over the same base provider and checks that each base-namespaced resource
// token resolves to its OWN extension's package reference. Because tokens live
// in the base namespace, resolving by the base name alone collapses both
// extensions onto one ref (last registration wins).
func TestPackageRefResolutionAcrossExtensionsOnSameBase(t *testing.T) {
	t.Parallel()

	baseVersion := semver.MustParse("45.0.0")
	extVersion := semver.MustParse("1.0.0")
	descriptor := func(extName string) *schema.PackageDescriptor {
		return &schema.PackageDescriptor{
			Name:    "extbase",
			Version: &baseVersion,
			Parameterization: &schema.ParameterizationDescriptor{
				Name:    extName,
				Version: extVersion,
				Value:   []byte(extName),
			},
		}
	}

	i := &Interpreter{
		monitor: registerPackageMonitor{},
		loader:  &multiExtensionRuntimeLoader{},
		info: RunInfo{
			PackageDescriptors: map[string]*schema.PackageDescriptor{
				"exta": descriptor("exta"),
				"extb": descriptor("extb"),
			},
		},
		packageRefs: map[string]string{},
	}

	require.NoError(t, i.registerPackages(t.Context()))

	ayeRef, err := i.getPackageRefFromToken("extbase:index:Aye")
	require.NoError(t, err)
	beeRef, err := i.getPackageRefFromToken("extbase:index:Bee")
	require.NoError(t, err)

	require.Equal(t, "ref-exta", ayeRef, "extbase:index:Aye is defined by the exta extension")
	require.Equal(t, "ref-extb", beeRef, "extbase:index:Bee is defined by the extb extension")
	require.NotEqual(t, ayeRef, beeRef,
		"resources from different extensions on the same base must not share a package ref")
}
