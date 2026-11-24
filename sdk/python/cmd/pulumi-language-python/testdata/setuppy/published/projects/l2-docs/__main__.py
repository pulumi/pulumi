import pulumi
import pulumi_docs as docs
import pulumi_enum as enum

enum_res = enum.Res("enumRes",
    int_enum=enum.IntEnum.INT_ONE,
    string_enum=enum.StringEnum.STRING_ONE)
res = docs.Resource("res",
    in_=docs.fun_output(in_=False).out,
    external_enum=enum.StringEnum.STRING_ONE)
