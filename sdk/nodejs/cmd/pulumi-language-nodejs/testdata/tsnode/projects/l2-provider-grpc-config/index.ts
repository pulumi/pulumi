import * as pulumi from "@pulumi/pulumi";
import * as testconfigprovider from "@pulumi/testconfigprovider";

// The schema provider covers interesting schema shapes.
const schemaprov = new testconfigprovider.Provider("schemaprov", {
    s1: "",
    s2: "x",
    s3: "{}",
    i1: 0,
    i2: 42,
    n1: 0,
    n2: 42.42,
    b1: true,
    b2: false,
    ls1: [],
    ls2: [
        "",
        "foo",
    ],
    li1: [
        1,
        2,
    ],
    ms1: {},
    ms2: {
        key1: "value1",
        key2: "value2",
    },
    mi1: {
        key1: 0,
        key2: 42,
    },
    os1: {},
    os2: {
        x: "x-value",
    },
    oi1: {
        x: 42,
    },
});
const schemaconf = new testconfigprovider.ConfigGetter("schemaconf", {}, {
    provider: schemaprov,
});
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
