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

package lifecycletest

import (
	"context"
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/deploytest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"

	lt "github.com/pulumi/pulumi/pkg/v3/engine/lifecycletest/framework"
)

// TestReproSnapshot reproduces a failing fuzz test using a hard-coded starting snapshot.
func TestTargetedUpdateRefreshUnknownChildProvider(t *testing.T) {
	t.Parallel()

	// TODO[pulumi/pulumi#22511]: Fix the underlying issue and re-enable this test.
	t.Skip("Skipping: targeted update with refresh produces unknown provider reference for child providers")

	p := &lt.TestPlan{
		Project: "test-project",
		Stack:   "test-stack",
	}
	project := p.GetProject()

	// Set up the initial snapshot.
	setupSnap := func() *deploy.Snapshot {
		s := &deploy.Snapshot{}

		prov0 := &resource.State{
			Type:   "pulumi:providers:pkg-mt06",
			URN:    "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-mt06::res-b0L9",
			Custom: true,
			ID:     "id-aQ2mk2A3Gc2W",
		}
		s.Resources = append(s.Resources, prov0)

		provRef0, err := providers.NewReference(prov0.URN, prov0.ID)
		require.NoError(t, err)

		res1 := &resource.State{
			Type:           "pkg-mt06:mod-qaO0:type-qit2",
			URN:            "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B",
			Custom:         false,
			ID:             "",
			Protect:        true,
			RetainOnDelete: true,
			Provider:       provRef0.String(),
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res1)

		prov2 := &resource.State{
			Type:   "pulumi:providers:pkg-aL11",
			URN:    "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pulumi:providers:pkg-aL11::res-daE8",
			Custom: true,
			ID:     "id-d5mE0e3du8zy",
			Parent: res1.URN,
		}
		s.Resources = append(s.Resources, prov2)

		provRef2, err := providers.NewReference(prov2.URN, prov2.ID)
		require.NoError(t, err)

		res3URN := "urn:pulumi:test-stack::test-project::" +
			"pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-dXI1:type-az15::res-q03W"
		res3 := &resource.State{
			Type:           "pkg-aL11:mod-dXI1:type-az15",
			URN:            resource.URN(res3URN),
			Custom:         false,
			ID:             "",
			Protect:        true,
			RetainOnDelete: true,
			Provider:       provRef2.String(),
			Parent:         res1.URN,
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res3)

		res4 := &resource.State{
			Type:           "pkg-aL11:mod-dXI1:type-az15",
			URN:            "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W",
			Custom:         false,
			Delete:         true,
			ID:             "",
			RetainOnDelete: true,
			Provider:       provRef2.String(),
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res4)

		res5URN := "urn:pulumi:test-stack::test-project::" +
			"pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1"
		res5 := &resource.State{
			Type:               "pkg-aL11:mod-gD82:type-x615",
			URN:                resource.URN(res5URN),
			Custom:             false,
			ID:                 "",
			Protect:            true,
			PendingReplacement: true,
			Provider:           provRef2.String(),
			Parent:             res1.URN,
			Inputs: resource.PropertyMap{
				"__id": resource.NewProperty(""),
			},
		}
		s.Resources = append(s.Resources, res5)

		return s
	}()
	require.NoError(t, setupSnap.VerifyIntegrity(), "initial snapshot is not valid")

	// Set up the reproduction providers and program.
	createF := func(ctx context.Context, req plugin.CreateRequest) (plugin.CreateResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.CreateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("create failure for %s", req.URN)
		}
		return plugin.CreateResponse{
			Properties: req.Properties,
			Status:     resource.StatusOK,
		}, nil
	}
	deleteF := func(ctx context.Context, req plugin.DeleteRequest) (plugin.DeleteResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-aL11::res-daE8":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-k7ME:type-lWJ0::res-a7eK":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-zRIm:type-v1Jd::res-oPfG":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-o903:type-fC14::res-u0Ji":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B":
			return plugin.DeleteResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("delete failure for %s", req.URN)
		}
		return plugin.DeleteResponse{
			Status: resource.StatusOK,
		}, nil
	}
	diffF := func(ctx context.Context, req plugin.DiffRequest) (plugin.DiffResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2::res-h66B":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.DiffResponse{}, fmt.Errorf("diff failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-mt06::res-b0L9":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: false,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-k7ME:type-lWJ0::res-a7eK":
			return plugin.DiffResponse{
				Changes:             plugin.DiffSome,
				ReplaceKeys:         []resource.PropertyKey{"__replace"},
				DeleteBeforeReplace: true,
			}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.DiffResponse{Changes: plugin.DiffSome}, nil
		}
		return plugin.DiffResponse{}, nil
	}
	readF := func(ctx context.Context, req plugin.ReadRequest) (plugin.ReadResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-o903:type-fC14::res-u0Ji":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-f2JG::res-cz8e":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pulumi:providers:pkg-aL11::res-daE8":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-aDm1:type-hu4N::res-rB12":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.ReadResponse{}, nil

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-zRIm:type-v1Jd::res-oPfG":
			return plugin.ReadResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("read failure for %s", req.URN)
		}
		return plugin.ReadResponse{
			ReadResult: plugin.ReadResult{Outputs: resource.PropertyMap{}},
			Status:     resource.StatusOK,
		}, nil
	}
	updateF := func(ctx context.Context, req plugin.UpdateRequest) (plugin.UpdateResponse, error) {
		switch req.URN {
		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-z1M1:type-m01D::res-f1FH":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a6Jt:type-fi4L::res-aE61":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-aL11:mod-dXI1:type-az15::res-q03W":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)

		case "urn:pulumi:test-stack::test-project::pkg-f2JG:mod-a0g6:type-joBK::res-a4hs":
			return plugin.UpdateResponse{
				Status: resource.StatusUnknown,
			}, fmt.Errorf("update failure for %s", req.URN)
		}
		return plugin.UpdateResponse{
			Properties: req.NewInputs,
			Status:     resource.StatusOK,
		}, nil
	}

	reproLoaders := []*deploytest.ProviderLoader{
		deploytest.NewProviderLoader("pkg-aL11", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
		deploytest.NewProviderLoader("pkg-f2JG", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
		deploytest.NewProviderLoader("pkg-mt06", semver.MustParse("1.0.0"), func() (plugin.Provider, error) {
			return &deploytest.Provider{
				CreateF: createF,
				DeleteF: deleteF,
				DiffF:   diffF,
				ReadF:   readF,
				UpdateF: updateF,
			}, nil
		}),
	}

	reproProgramF := deploytest.NewLanguageRuntimeF(func(_ plugin.RunInfo, monitor *deploytest.ResourceMonitor) error {
		prov0, err := monitor.RegisterResource("pulumi:providers:pkg-f2JG", "res-cz8e", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		provRef0, err := providers.NewReference(prov0.URN, prov0.ID)
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-a0g6:type-joBK", "res-a4hs", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		res2, err := monitor.RegisterResource("pkg-f2JG:mod-z1M1:type-m01D", "res-f1FH", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkg-mt06", "res-b0L9", true, deploytest.ResourceOptions{})
		require.NoError(t, err)

		res4, err := monitor.RegisterResource("pkg-f2JG:mod-a6Jt:type-fi4L", "res-aE61", false, deploytest.ResourceOptions{
			Protect:        ptr(true),
			RetainOnDelete: ptr(true),
			Provider:       provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pulumi:providers:pkg-aL11", "res-daE8", true, deploytest.ResourceOptions{
			Protect:        ptr(true),
			RetainOnDelete: ptr(true),
		})
		require.NoError(t, err)

		res6, err := monitor.RegisterResource("pkg-aL11:mod-dXI1:type-az15", "res-q03W", false, deploytest.ResourceOptions{
			Protect: ptr(true),
			AliasURNs: []resource.URN{
				"urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-dXI1:type-az15::res-q03W",
			},
		})
		require.NoError(t, err)

		res7, err := monitor.RegisterResource("pkg-f2JG:mod-zRIm:type-v1Jd", "res-oPfG", true, deploytest.ResourceOptions{
			RetainOnDelete: ptr(true),
			Provider:       provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-k7ME:type-lWJ0", "res-a7eK", false, deploytest.ResourceOptions{
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-aDm1:type-hu4N", "res-rB12", true, deploytest.ResourceOptions{
			Provider: provRef0.String(),
			Dependencies: []resource.URN{
				res4.URN,
				res6.URN,
				res7.URN,
			},
			DeletedWith: res2.URN,
		})
		require.NoError(t, err)

		_, err = monitor.RegisterResource("pkg-f2JG:mod-o903:type-fC14", "res-u0Ji", false, deploytest.ResourceOptions{
			Protect:  ptr(true),
			Provider: provRef0.String(),
		})
		require.NoError(t, err)

		return nil
	})

	reproHostF := deploytest.NewPluginHostF(nil, nil, reproProgramF, reproLoaders...)
	reproOpts := lt.TestUpdateOptions{
		T:     t,
		HostF: reproHostF,
		UpdateOptions: engine.UpdateOptions{
			Refresh: true,
			Targets: deploy.NewUrnTargets([]string{
				"urn:pulumi:test-stack::test-project::pkg-mt06:mod-qaO0:type-qit2$pkg-aL11:mod-gD82:type-x615::res-t5q1",
			}),
		},
	}

	// Trigger the reproduction.
	_, err := lt.TestOp(engine.Update).RunStep(
		project, p.GetTarget(t, setupSnap), reproOpts, false, p.BackendClient, nil, "1")
	require.NoError(t, err)
}
