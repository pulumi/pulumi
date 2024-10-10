import * as pulumi from "@pulumi/pulumi";
import * as testconfigprovider from "@pulumi/testconfigprovider";

// The program_secret_provider covers scenarios where user passes secret values to the provider.
const programsecretprov = new testconfigprovider.Provider("programsecretprov", {
    s1: testconfigprovider.toSecretOutput({
        s: "SECRET",
    }).apply(invoke => invoke.s),
    i1: testconfigprovider.toSecretOutput({
        i: 1234567890,
    }).apply(invoke => invoke.i),
    n1: testconfigprovider.toSecretOutput({
        n: 123456.789,
    }).apply(invoke => invoke.n),
    b1: testconfigprovider.toSecretOutput({
        b: true,
    }).apply(invoke => invoke.b),
    ls1: testconfigprovider.toSecretOutput({
        ls: [
            "SECRET",
            "SECRET2",
        ],
    }).apply(invoke => invoke.ls),
    ls2: [
        "VALUE",
        testconfigprovider.toSecretOutput({
            s: "SECRET",
        }).apply(invoke => invoke.s),
    ],
    ms2: {
        key1: "value1",
        key2: testconfigprovider.toSecretOutput({
            s: "SECRET",
        }).apply(invoke => invoke.s),
    },
    os2: {
        x: testconfigprovider.toSecretOutput({
            s: "SECRET",
        }).apply(invoke => invoke.s),
    },
});
const programsecretconf = new testconfigprovider.ConfigGetter("programsecretconf", {}, {
    provider: programsecretprov,
});
