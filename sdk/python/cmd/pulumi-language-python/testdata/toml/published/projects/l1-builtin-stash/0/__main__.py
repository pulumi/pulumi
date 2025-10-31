import pulumi

my_stash = pulumi.Stash("myStash", value={
    "key": [
        "value",
        "s",
    ],
    "": False,
})
pulumi.export("stashOutput", my_stash.value)
passthrough_stash = pulumi.Stash("passthroughStash",
    value="old",
    passthrough=True)
pulumi.export("passthroughOutput", passthrough_stash.value)
