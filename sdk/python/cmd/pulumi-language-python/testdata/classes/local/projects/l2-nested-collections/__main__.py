import pulumi
import pulumi_nestedcollections as nestedcollections

# A resource with deeply nested collection output properties: a list of lists of lists
# of an object type and a map of maps of maps of strings.
foo = nestedcollections.Foo("foo")
pulumi.export("secondProp", foo.condition_sets[0][0][1].prop)
pulumi.export("leaf", foo.private_endpoint["outer"]["inner"]["leaf"])
