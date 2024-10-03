import pulumi
import pulumi_random as random

foo = random.RandomPet("foo", opts = pulumi.ResourceOptions(retain_on_delete=True))
