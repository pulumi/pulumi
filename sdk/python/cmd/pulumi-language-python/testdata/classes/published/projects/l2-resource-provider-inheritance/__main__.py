import pulumi
import pulumi_primitive as primitive
import pulumi_simple as simple

provider = simple.Provider("provider")
parent1 = simple.Resource("parent1", value=True,
opts = pulumi.ResourceOptions(provider=provider))
# This should inherit the explicit provider from parent1
child1 = simple.Resource("child1", value=True,
opts = pulumi.ResourceOptions(parent=parent1))
parent2 = primitive.Resource("parent2",
    boolean=False,
    float=float(0),
    integer=0,
    string="",
    number_array=[],
    boolean_map={})
# This _should not_ inherit the provider from parent2 as it is a default provider.
child2 = simple.Resource("child2", value=True,
opts = pulumi.ResourceOptions(parent=parent2))
# This _should not_ inherit the provider from parent1 as its from the wrong package.
child3 = primitive.Resource("child3",
    boolean=False,
    float=float(0),
    integer=0,
    string="",
    number_array=[],
    boolean_map={},
    opts = pulumi.ResourceOptions(parent=parent1))
