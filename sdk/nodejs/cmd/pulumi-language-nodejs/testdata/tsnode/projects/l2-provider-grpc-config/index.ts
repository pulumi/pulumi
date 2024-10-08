import * as pulumi from "@pulumi/pulumi";
import * as testconfigprovider from "@pulumi/testconfigprovider";

const prov1 = new testconfigprovider.Provider("prov1", {
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
const c1 = new testconfigprovider.ConfigGetter("c1", {}, {
    provider: prov1,
});
