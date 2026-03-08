import pulumi
import pulumi_enum as enum
import pulumi_extenumref as extenumref

my_res = enum.Res("myRes",
    int_enum=enum.IntEnum.INT_ONE,
    string_enum=enum.StringEnum.STRING_ONE)
my_sink = extenumref.Sink("mySink", string_enum=enum.StringEnum.STRING_TWO)
