import pulumi
import pulumi_config_grpc as config_grpc

# This provider covers scenarios where configuration properties are marked as secret in the schema.
config_grpc_provider = config_grpc.Provider("config_grpc_provider", secret_string1="SECRET")
# listSecretString1 = ["SECRET", "SECRET2"]
# mapSecretString1 = { key1 = "SECRET", key2 = "SECRET2" }
# objSecretString1 = { x = "SECRET" }
config = config_grpc.ConfigFetcher("config", opts = pulumi.ResourceOptions(provider=config_grpc_provider))
