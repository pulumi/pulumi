import pulumi
import pulumi_std as std

example = std.replace(text=std.upper(input="hello_world").result,
    search="_",
    replace="-")
