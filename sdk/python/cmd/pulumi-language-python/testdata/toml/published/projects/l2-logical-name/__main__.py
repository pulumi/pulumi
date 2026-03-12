import pulumi
import pulumi_simple as simple

config = pulumi.Config()
config_lexical_name = config.require_bool("cC-Charlie_charlie.😃⁉️")
resource_lexical_name = simple.Resource("aA-Alpha_alpha.🤯⁉️", value=config_lexical_name)
pulumi.export("bB-Beta_beta.💜⁉", resource_lexical_name.value)
pulumi.export("dD-Delta_delta.🔥⁉", resource_lexical_name.value)
