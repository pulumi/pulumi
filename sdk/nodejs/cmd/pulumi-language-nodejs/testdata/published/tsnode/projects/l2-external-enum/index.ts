import * as pulumi from "@pulumi/pulumi";
import * as _enum from "@pulumi/enum";
import * as extenumref from "@pulumi/extenumref";

const myRes = new _enum.Res("myRes", {
    intEnum: _enum.IntEnum.IntOne,
    stringEnum: _enum.StringEnum.StringOne,
});
const mySink = new extenumref.Sink("mySink", {stringEnum: _enum.StringEnum.StringTwo});
