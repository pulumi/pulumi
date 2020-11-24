package main

import (
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi-random/sdk/v2/go/random"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
)

type MyResource struct {
	pulumi.ResourceState

	Length pulumi.IntOutput
}

type myResourceArgs struct{}
type MyResourceArgs struct{}

func (MyResourceArgs) ElementType() reflect.Type {
	return reflect.TypeOf((*myResourceArgs)(nil)).Elem()
}

func GetResource(ctx *pulumi.Context, urn pulumi.URN) (*MyResource, error) {
	var resource MyResource
	err := ctx.RegisterResource("unused:unused:unused", "unused", &MyResourceArgs{}, &resource,
		pulumi.URN_(string(urn)))
	fmt.Printf("resource %#v", resource)
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		pet, err := random.NewRandomPet(ctx, "cat", &random.RandomPetArgs{})
		if err != nil {
			return err
		}

		getPetLength := pet.URN().ApplyT(func(urn pulumi.URN) (pulumi.IntOutput, error) {
			//fmt.Printf("urn %s", urn)
			//return pulumi.IntOutput{}, nil
			r, err := GetResource(ctx, urn)
			if err != nil {
				fmt.Printf("error: %s", err)
				return pulumi.IntOutput{}, err
			}
			fmt.Printf("length: %v", r)
			return r.Length.ToIntOutput(), nil
		})
		ctx.Export("getPetLength", getPetLength)

		return nil
	})
}
