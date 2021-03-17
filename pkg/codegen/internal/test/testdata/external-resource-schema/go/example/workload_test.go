package example

import (
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type mocks int

func (mocks) NewResource(
	typeToken string,
	name string,
	inputs resource.PropertyMap,
	provider string,
	id string,
) (string, resource.PropertyMap, error) {
	return name + "_id", inputs, nil
}

func (mocks) Call(token string, args resource.PropertyMap, provider string) (resource.PropertyMap, error) {
	return args, nil
}

func TestResourceCollectionOutputs(t *testing.T) {
	t.Run("ArrayOutput", func(t *testing.T) {
		require.NoError(t, pulumi.RunErr(func(ctx *pulumi.Context) error {
			workload1, err := NewWorkload(ctx, "workload1", nil)
			require.NoError(t, err)
			workload2, err := NewWorkload(ctx, "workload2", nil)
			require.NoError(t, err)
			workloadArr := WorkloadArray{
				workload1,
				workload2,
			}.ToWorkloadArrayOutput()
			var wg sync.WaitGroup
			wg.Add(1)
			pulumi.All(workloadArr.Index(pulumi.Int(0)), workloadArr.Index(pulumi.Int(1))).
				ApplyT(func(all []interface{}) error {
					w1 := all[0].(*Workload)
					w2 := all[1].(*Workload)
					assert.Equal(t, w1.URN(), workload1.URN())
					assert.Equal(t, w2.URN(), workload2.URN())
					wg.Done()
					return nil
				})
			wg.Wait()
			return nil
		}, pulumi.WithMocks("project", "stack", mocks(1))))
	})
}
