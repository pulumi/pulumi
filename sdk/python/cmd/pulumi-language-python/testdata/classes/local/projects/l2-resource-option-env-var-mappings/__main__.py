import pulumi
import pulumi_simple as simple

prov = simple.Provider("prov", opts = pulumi.ResourceOptions(env_var_mappings={
        "MY_VAR": "PROVIDER_VAR",
        "OTHER_VAR": "TARGET_VAR",
    }))
