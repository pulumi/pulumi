import pulumi
import pulumi_random as random

resource_lexical_name = random.RandomPet("aA-Alpha_alpha.🤯⁉️")
pulumi.export("bB-Beta_beta.💜⁉", resource_lexical_name.id)
