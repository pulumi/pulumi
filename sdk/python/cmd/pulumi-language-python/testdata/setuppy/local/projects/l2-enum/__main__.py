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
