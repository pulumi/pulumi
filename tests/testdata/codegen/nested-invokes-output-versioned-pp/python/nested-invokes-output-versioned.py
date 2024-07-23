import pulumi
import pulumi_std as std

example = std.replace_output(text=std.upper_output(input="hello_world").apply(lambda invoke: invoke.result),
    search="_",
    replace="-")
