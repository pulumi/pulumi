import * as pulumi from "@pulumi/pulumi";
import * as config_grpc from "@pulumi/config-grpc";

// Cover interesting schema shapes.
const config_grpc_provider = new config_grpc.Provider("config_grpc_provider", {
    string1: "",
    string2: "x",
    string3: "{}",
    int1: 0,
    int2: 42,
    num1: 0,
    num2: 42.42,
    bool1: true,
    bool2: false,
    listString1: [],
    listString2: [
        "",
        "foo",
    ],
    listInt1: [
        1,
        2,
    ],
    mapString1: {},
    mapString2: {
        key1: "value1",
        key2: "value2",
    },
    mapInt1: {
        key1: 0,
        key2: 42,
    },
    objString1: {},
    objString2: {
        x: "x-value",
    },
    objInt1: {
        x: 42,
    },
});
const config = new config_grpc.ConfigFetcher("config", {}, {
    provider: config_grpc_provider,
});
