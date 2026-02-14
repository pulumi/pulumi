import pulumi
import pulumi_discriminated_union as discriminated_union

cat_example = discriminated_union.Example("catExample",
    pet=discriminated_union.CatArgs(
        pet_type="cat",
        meow="meow",
    ),
    pets=[discriminated_union.CatArgs(
        pet_type="cat",
        meow="purr",
    )])
dog_example = discriminated_union.Example("dogExample",
    pet=discriminated_union.DogArgs(
        pet_type="dog",
        bark="woof",
    ),
    pets=[
        discriminated_union.DogArgs(
            pet_type="dog",
            bark="bark",
        ),
        discriminated_union.CatArgs(
            pet_type="cat",
            meow="hiss",
        ),
    ])
