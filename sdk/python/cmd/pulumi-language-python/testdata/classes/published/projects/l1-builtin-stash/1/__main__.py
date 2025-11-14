import pulumi

my_stash = pulumi.Stash("myStash", input="ignored")
pulumi.export("stashInput", my_stash.input)
pulumi.export("stashOutput", my_stash.output)
passthrough_stash = pulumi.Stash("passthroughStash",
    input="new",
    passthrough=True)
pulumi.export("passthroughInput", passthrough_stash.input)
pulumi.export("passthroughOutput", passthrough_stash.output)
