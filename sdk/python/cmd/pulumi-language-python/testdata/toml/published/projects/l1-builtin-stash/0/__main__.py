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
passthrough_stash = pulumi.Stash("passthroughStash",
    input="old",
    passthrough=True)
pulumi.export("passthroughInput", passthrough_stash.input)
pulumi.export("passthroughOutput", passthrough_stash.output)
