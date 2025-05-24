package tests

import (
	"resource-property-overlap/example"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Tests that XArray{x}.ToXArrayOutput().Index(pulumi.Int(0)) == x.
func TestArrayOutputIndex(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		r1, err := example.NewRec(ctx, "rec1", &example.RecArgs{})
		if err != nil {
			return err
		}

		r1o := r1.ToRecOutput()

		r2o := example.RecArray{r1o}.ToRecArrayOutput().
			Index(pulumi.Int(0))

		wg := &sync.WaitGroup{}
		wg.Add(1)

		pulumi.All(r1o, r2o).ApplyT(func(xs []interface{}) int {
			assert.Equal(t, xs[0], xs[1])
			wg.Done()
			return 0
		})

		wg.Wait()
		return nil
	}, pulumi.WithMocks("project", "stack", mocks(0)))
	assert.NoError(t, err)
}

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) MethodCall(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}

func (mocks) Call(args pulumi.MockInvokeArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
