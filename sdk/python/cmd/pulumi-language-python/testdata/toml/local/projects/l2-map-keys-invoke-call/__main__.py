import pulumi
import pulumi_component as component
import pulumi_simple_invoke as simple_invoke

callable = component.ComponentCallable("callable", value="unused")
pulumi.export("invokeResult", simple_invoke.echo_map_output(string_map={
    "my key": "one",
    "my.key": "two",
    "my-key": "three",
    "my_key": "four",
    "MY_KEY": "five",
    "myKey": "six",
    "__type": "seven",
    "__internal": "eight",
}).string_map)
pulumi.export("callResult", callable.echo_map(string_map={
    "my key": "one",
    "my.key": "two",
    "my-key": "three",
    "my_key": "four",
    "MY_KEY": "five",
    "myKey": "six",
    "__type": "seven",
    "__internal": "eight",
}).string_map)
