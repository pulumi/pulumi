package main

import (
	"example.com/pulumi-discriminated-union/sdk/go/v30/discriminatedunion"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := discriminatedunion.NewExample(ctx, "catExample", &discriminatedunion.ExampleArgs{
			Pet: &discriminatedunion.CatArgs{
				PetType: pulumi.String("cat"),
				Meow:    pulumi.String("meow"),
			},
			Pets: pulumi.Array{
				discriminatedunion.Cat{
					PetType: "cat",
					Meow:    "purr",
				},
			},
		})
		if err != nil {
			return err
		}
		_, err = discriminatedunion.NewExample(ctx, "dogExample", &discriminatedunion.ExampleArgs{
			Pet: &discriminatedunion.DogArgs{
				PetType: pulumi.String("dog"),
				Bark:    pulumi.String("woof"),
			},
			Pets: pulumi.Array{
				discriminatedunion.Dog{
					PetType: "dog",
					Bark:    "bark",
				},
				discriminatedunion.Cat{
					PetType: "cat",
					Meow:    "hiss",
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
