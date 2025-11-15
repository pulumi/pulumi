import pulumi

my_stash = pulumi.Stash("myStash", input={
    "key": [
        "value",
        "s",
    ],
    "": False,
})
pulumi.export("stashInput", my_stash.input)
pulumi.export("stashOutput", my_stash.output)
