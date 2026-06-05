import * as pulumi from "@pulumi/pulumi";
import * as _enum from "@pulumi/enum";
import * as docs from "@pulumi/docs";

const enumRes = new _enum.Res("enumRes", {
    intEnum: _enum.IntEnum.IntOne,
    stringEnum: _enum.StringEnum.StringOne,
});
const res = new docs.Resource("res", {
    "in": docs.funOutput({
        "in": false,
    }).apply(invoke => invoke.out),
    externalEnum: _enum.StringEnum.StringOne,
});
