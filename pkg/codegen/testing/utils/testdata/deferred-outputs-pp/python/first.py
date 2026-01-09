import pulumi
from pulumi import Input
from typing import Optional, Dict, TypedDict, Any
import pulumi_random as random

class FirstArgs(TypedDict, total=False):
    passwordLength: Input[float]

class First(pulumi.ComponentResource):
    def __init__(self, name: str, args: FirstArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:First", name, args, opts)

        random_pet = random.RandomPet(f"{name}-randomPet", opts = pulumi.ResourceOptions(parent=self))

        random_password = random.RandomPassword(f"{name}-randomPassword", length=args["passwordLength"],
        opts = pulumi.ResourceOptions(parent=self))

        self.petName = random_pet.id
        self.password = random_password.result
        self.register_outputs({
            'petName': random_pet.id, 
            'password': random_password.result
        })