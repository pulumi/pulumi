import pulumi
import pulumi_random as random

config = pulumi.Config()
config_lexical_name = config.require("cC-Charlie_charlie.😃⁉️")
resource_lexical_name = random.RandomPet("aA-Alpha_alpha.🤯⁉️", prefix=config_lexical_name)
pulumi.export("bB-Beta_beta.💜⁉", resource_lexical_name.id)
pulumi.export("dD-Delta_delta.🔥⁉", resource_lexical_name.id)
