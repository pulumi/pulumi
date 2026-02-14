import pulumi
import pulumi_discriminated_union as discriminated_union

cat_example = discriminated_union.Example("catExample",
    pet={
        "pet_type": "cat",
        "meow": "meow",
    },
    pets=[{
        "pet_type": "cat",
        "meow": "purr",
    }])
dog_example = discriminated_union.Example("dogExample",
    pet={
        "pet_type": "dog",
        "bark": "woof",
    },
    pets=[
        {
            "pet_type": "dog",
            "bark": "bark",
        },
        {
            "pet_type": "cat",
            "meow": "hiss",
        },
    ])
