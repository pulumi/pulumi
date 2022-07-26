package main

import (
	"reflect"

	"github.com/pulumi/pulumi-random/sdk/v3/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type MyResource struct {
	pulumi.ResourceState

	Length pulumi.IntOutput       `pulumi:"length"`
	Prefix pulumi.StringPtrOutput `pulumi:"prefix"`
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
	if err != nil {
		return nil, err
	}
	return &resource, nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		c := config.New(ctx, "")
		bar := c.RequireSecret("bar")
		pet, err := random.NewRandomPet(ctx, "cat", &random.RandomPetArgs{
			Length: pulumi.Int(2),
			Prefix: bar,
		})
		if err != nil {
			return err
		}

		getPetLength := pet.URN().ApplyT(func(urn pulumi.URN) (pulumi.IntInput, error) {
			r, err := GetResource(ctx, urn)
			if err != nil {
				return nil, err
			}
			return r.Length, nil
		})
		getPetSecret := pet.URN().ApplyT(func(urn pulumi.URN) (pulumi.StringPtrInput, error) {
			r, err := GetResource(ctx, urn)
			if err != nil {
				return nil, err
			}
			return r.Prefix, nil
		})
		ctx.Export("getPetLength", getPetLength)
		ctx.Export("secret", getPetSecret)

		return nil
	})
}
