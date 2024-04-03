import pulumi
import pulumi_random as random

random_password = random.RandomPassword("randomPassword",
    length=16,
    special=True,
    override_special="_%@")
pulumi.export("password", random_password.result)
