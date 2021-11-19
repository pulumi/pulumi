package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"resource-property-overlap/example"
)

func TestArrayOutputIndex(t *testing.T) {
	err := pulumi.RunErr(func(ctx *pulumi.Context) error {
		rec, err := example.NewRec(ctx, "rec", &example.RecArgs{})
		if err != nil {
			return err
		}

		var u1, u2 pulumi.URNOutput

		u1 = rec.URN()

		u2 = example.RecArray{rec}.ToRecArrayOutput().
			Index(pulumi.Int(0)).
			ApplyT(func(rec2 *example.Rec) pulumi.URNOutput {
				return rec2.URN()
			}).
			ApplyT(func(x interface{}) pulumi.URN {
				return x.(pulumi.URN)
			}).(pulumi.URNOutput)

		wg := &sync.WaitGroup{}
		wg.Add(1)

		pulumi.All(u1, u2).ApplyT(func(all []interface{}) int {
			urn1 := all[0].(pulumi.URN)
			urn2 := all[2].(pulumi.URN)
			assert.Equal(t, urn1, urn2)
			wg.Done()
			return 0
		})

		return waitOrTimeout(wg)

	}, pulumi.WithMocks("project", "stack", mocks(1)))
	assert.NoError(t, err)
}

func waitOrTimeout(wg *sync.WaitGroup) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return nil
	case <-time.After(1 * time.Second):
		return fmt.Errorf("Timeout")
	}

}

type mocks int

func (mocks) NewResource(args pulumi.MockResourceArgs) (string, resource.PropertyMap, error) {
	return args.Name + "_id", args.Inputs, nil
}

func (mocks) Call(args pulumi.MockCallArgs) (resource.PropertyMap, error) {
	return args.Args, nil
}
