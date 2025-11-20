"""A Random Python Pulumi program"""

import pulumi
import pulumi_random as random

username = random.RandomPet('username')

pulumi.export('name', username.id)
