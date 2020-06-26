import pulumi
import pulumi_random as random

random_pet = random.RandomPet("random_pet", prefix="doggo")
