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
const sink4 = new _enum.Deluxe("sink4", {
    numberEnum: _enum.NumberEnum.ZeroPointOne,
    wordyEnum: _enum.WordyEnum.It_s_got_apostrophes,
    arrayOfEnum: [
        _enum.StringEnum.StringOne,
        _enum.StringEnum.StringTwo,
    ],
    mapOfEnum: {
        small: _enum.IntEnum.IntOne,
        large: _enum.IntEnum.IntTwo,
    },
    holder: {
        size: _enum.IntEnum.IntTwo,
        color: _enum.StringEnum.StringOne,
    },
    unionEnum: _enum.WordyEnum.A_Value_With_Spaces_,
});
