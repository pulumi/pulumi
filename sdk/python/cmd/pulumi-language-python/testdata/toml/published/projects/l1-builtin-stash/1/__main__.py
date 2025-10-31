import pulumi

my_stash = pulumi.Stash("myStash", value="ignored")
pulumi.export("stashOutput", my_stash.value)
passthrough_stash = pulumi.Stash("passthroughStash",
    value="new",
    passthrough=True)
pulumi.export("passthroughOutput", passthrough_stash.value)
