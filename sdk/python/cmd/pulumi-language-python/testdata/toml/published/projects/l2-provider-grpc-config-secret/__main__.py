import pulumi
import pulumi_config_grpc as config_grpc

# This provider covers scenarios where user passes secret values to the provider.
config_grpc_provider = config_grpc.Provider("config_grpc_provider",
    string1=config_grpc.to_secret_output(string1="SECRET").string1,
    int1=config_grpc.to_secret_output(int1=1234567890).int1,
    num1=config_grpc.to_secret_output(num1=123456.789).num1,
    bool1=config_grpc.to_secret_output(bool1=True).bool1,
    list_string1=config_grpc.to_secret_output(list_string1=[
        "SECRET",
        "SECRET2",
    ]).list_string1,
    list_string2=[
        "VALUE",
        config_grpc.to_secret_output(string1="SECRET").string1,
    ],
    map_string2={
        "key1": "value1",
        "key2": config_grpc.to_secret_output(string1="SECRET").string1,
    },
    obj_string2={
        "x": config_grpc.to_secret_output(string1="SECRET").string1,
    })
config = config_grpc.ConfigFetcher("config", opts = pulumi.ResourceOptions(provider=config_grpc_provider))
