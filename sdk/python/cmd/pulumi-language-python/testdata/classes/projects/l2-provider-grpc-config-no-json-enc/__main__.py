import pulumi
import pulumi_config_grpc as config_grpc

# Cover interesting schema shapes.
config_grpc_provider = config_grpc.Provider("config_grpc_provider",
    string1="",
    string2="x",
    string3="{}",
    int1=0,
    int2=42,
    num1=0,
    num2=42.42,
    bool1=True,
    bool2=False,
    list_string1=[],
    list_string2=[
        "",
        "foo",
    ],
    list_int1=[
        1,
        2,
    ],
    map_string1={},
    map_string2={
        "key1": "value1",
        "key2": "value2",
    },
    map_int1={
        "key1": 0,
        "key2": 42,
    },
    obj_string1=config_grpc.Tstring1Args(),
    obj_string2=config_grpc.Tstring2Args(
        x="x-value",
    ),
    obj_int1=config_grpc.Tint1Args(
        x=42,
    ))
config = config_grpc.ConfigFetcher("config", opts = pulumi.ResourceOptions(provider=config_grpc_provider))
