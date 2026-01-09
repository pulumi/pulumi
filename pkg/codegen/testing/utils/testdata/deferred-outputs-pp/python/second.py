import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_random as random

class SecondArgs(TypedDict, total=False):
    petName: Input[str]

class Second(pulumi.ComponentResource):
    def __init__(self, name: str, args: SecondArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:Second", name, args, opts)

        random_pet = random.RandomPet(f"{name}-randomPet", length=len(args["petName"]),
        opts = pulumi.ResourceOptions(parent=self))

        password = random.RandomPassword(f"{name}-password",
            length=16,
            special=True,
            numeric=False,
            opts = pulumi.ResourceOptions(parent=self))

        self.passwordLength = password.length
        self.register_outputs({
            'passwordLength': password.length
        })