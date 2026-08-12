import pulumi

my_stash = pulumi.Stash("myStash", value={
    "key": [
        "value",
        "s",
    ],
    "": False,
})
pulumi.export("stashOutput", my_stash.value)
