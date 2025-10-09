import pulumi
import pulumi_simple as simple

provider = simple.Provider("provider")
parent1 = simple.Resource("parent1", value=True,
opts = pulumi.ResourceOptions(provider=provider))
child1 = simple.Resource("child1", value=True,
opts = pulumi.ResourceOptions(parent=parent1))
orphan1 = simple.Resource("orphan1", value=True)
parent2 = simple.Resource("parent2", value=True,
opts = pulumi.ResourceOptions(protect=True,
    retain_on_delete=True))
child2 = simple.Resource("child2", value=True,
opts = pulumi.ResourceOptions(parent=parent2))
child3 = simple.Resource("child3", value=True,
opts = pulumi.ResourceOptions(parent=parent2,
    protect=False,
    retain_on_delete=False))
orphan2 = simple.Resource("orphan2", value=True)
