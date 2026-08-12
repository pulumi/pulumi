import pulumi
import pulumi_config_grpc as config_grpc

# This provider covers scenarios where configuration properties are marked as secret in the schema.
config_grpc_provider = config_grpc.Provider("config_grpc_provider",
    secret_string1="SECRET",
    secret_int1=16,
    secret_num1=123456.789,
    secret_bool1=True,
    list_secret_string1=[
        "SECRET",
        "SECRET2",
    ],
    map_secret_string1={
        "key1": "SECRET",
        "key2": "SECRET2",
    },
    obj_secret_string1={
        "secret_x": "SECRET",
    })
config = config_grpc.ConfigFetcher("config", opts = pulumi.ResourceOptions(provider=config_grpc_provider))
