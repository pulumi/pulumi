import pulumi
import pulumi_enum as enum

sink1 = enum.Res("sink1",
    int_enum=enum.IntEnum.INT_ONE,
    string_enum=enum.StringEnum.STRING_TWO)
sink2 = enum.mod.Res("sink2",
    int_enum=enum.mod.IntEnum.INT_ONE,
    string_enum=enum.mod.StringEnum.STRING_TWO)
sink3 = enum.mod.nested.Res("sink3",
    int_enum=enum.mod.nested.IntEnum.INT_ONE,
    string_enum=enum.mod.nested.StringEnum.STRING_TWO)
sink4 = enum.Deluxe("sink4",
    number_enum=enum.NumberEnum.ZERO_POINT_ONE,
    wordy_enum=enum.WordyEnum.IT_S_GOT_APOSTROPHES,
    array_of_enum=[
        enum.StringEnum.STRING_ONE,
        enum.StringEnum.STRING_TWO,
    ],
    map_of_enum={
        "small": enum.IntEnum.INT_ONE,
        "large": enum.IntEnum.INT_TWO,
    },
    holder=enum.HolderArgs(
        size=enum.IntEnum.INT_TWO,
        color=enum.StringEnum.STRING_ONE,
    ),
    union_enum=enum.WordyEnum.A_VALUE_WITH_SPACES_)
