import * as pulumi from "@pulumi/pulumi";
import * as discriminated_union from "@pulumi/discriminated-union";

const example1 = new discriminated_union.Example("example1", {
    unionOf: {
        discriminantKind: "variant1",
        field1: "v1 union",
    },
    arrayOfUnionOf: [{
        discriminantKind: "variant1",
        field1: "v1 array(union)",
    }],
});
const example2 = new discriminated_union.Example("example2", {
    unionOf: {
        discriminantKind: "variant2",
        field2: "v2 union",
    },
    arrayOfUnionOf: [
        {
            discriminantKind: "variant2",
            field2: "v2 array(union)",
        },
        {
            discriminantKind: "variant1",
            field1: "v1 array(union)",
        },
    ],
});
