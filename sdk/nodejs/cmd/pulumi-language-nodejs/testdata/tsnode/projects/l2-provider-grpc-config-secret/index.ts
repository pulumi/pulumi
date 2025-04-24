import * as pulumi from "@pulumi/pulumi";
import * as config_grpc from "@pulumi/config-grpc";

// This provider covers scenarios where user passes secret values to the provider.
const config_grpc_provider = new config_grpc.Provider("config_grpc_provider", {
    string1: config_grpc.toSecretOutput({
        string1: "SECRET",
    }).apply(invoke => invoke.string1),
    int1: config_grpc.toSecretOutput({
        int1: 1234567890,
    }).apply(invoke => invoke.int1),
    num1: config_grpc.toSecretOutput({
        num1: 123456.789,
    }).apply(invoke => invoke.num1),
    bool1: config_grpc.toSecretOutput({
        bool1: true,
    }).apply(invoke => invoke.bool1),
    listString1: config_grpc.toSecretOutput({
        listString1: [
            "SECRET",
            "SECRET2",
        ],
    }).apply(invoke => invoke.listString1),
    listString2: [
        "VALUE",
        config_grpc.toSecretOutput({
            string1: "SECRET",
        }).apply(invoke => invoke.string1),
    ],
    mapString2: {
        key1: "value1",
        key2: config_grpc.toSecretOutput({
            string1: "SECRET",
        }).apply(invoke => invoke.string1),
    },
    objString2: {
        x: config_grpc.toSecretOutput({
            string1: "SECRET",
        }).apply(invoke => invoke.string1),
    },
});
const config = new config_grpc.ConfigFetcher("config", {}, {
    provider: config_grpc_provider,
});
