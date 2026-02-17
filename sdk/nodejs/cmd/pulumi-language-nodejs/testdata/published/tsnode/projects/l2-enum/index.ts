import * as pulumi from "@pulumi/pulumi";
import * as _enum from "@pulumi/enum";

const sink1 = new _enum.Res("sink1", {
    intEnum: _enum.IntEnum.IntOne,
    stringEnum: _enum.StringEnum.StringTwo,
});
const sink2 = new _enum.mod.Res("sink2", {
    intEnum: _enum.mod.IntEnum.IntOne,
    stringEnum: _enum.mod.StringEnum.StringTwo,
});
const sink3 = new _enum.mod.nested.Res("sink3", {
    intEnum: _enum.mod.nested.IntEnum.IntOne,
    stringEnum: _enum.mod.nested.StringEnum.StringTwo,
});
