import pulumi
from pulumi import Input
from simpleComponent import SimpleComponent
from typing import Optional, Dict, TypedDict, Any
from typing_extensions import NotRequired
import builtins as _builtins
import pulumi_random as random

class DeploymentZones(TypedDict):
    zone: NotRequired[Input[_builtins.str]]

class GithubApp(TypedDict):
    id: NotRequired[Input[_builtins.str]]
    keyBase64: NotRequired[Input[_builtins.str]]
    webhookSecret: NotRequired[Input[_builtins.str]]

class Servers(TypedDict):
    name: NotRequired[Input[_builtins.str]]

class ExampleComponentArgs(TypedDict):
    input: Input[_builtins.str]
    cidrBlocks: Input[Dict[_builtins.str, _builtins.str]]
    githubApp: NotRequired[Input[GithubApp]]
    servers: NotRequired[Input[list(Servers)]]
    deploymentZones: NotRequired[Input[Dict[str, DeploymentZones]]]
    ipAddress: Input[list[_builtins.int]]

class ExampleComponent(pulumi.ComponentResource):
    def __init__(self, name: str, args: ExampleComponentArgs, opts:Optional[pulumi.ResourceOptions] = None):
        super().__init__("components:index:ExampleComponent", name, args, opts)

        password = random.RandomPassword(f"{name}-password",
            length=16,
            special=True,
            override_special=args["input"],
            opts = pulumi.ResourceOptions(parent=self))

        github_password = random.RandomPassword(f"{name}-githubPassword",
            length=16,
            special=True,
            override_special=args["githubApp"]["webhookSecret"],
            opts = pulumi.ResourceOptions(parent=self))

        # Example of iterating a list of objects
        server_passwords: list[random.RandomPassword] = []
        for server_passwords_range in [{"value": i} for i in range(0, len(args["servers"]))]:
            server_passwords.append(random.RandomPassword(f"{name}-serverPasswords-{server_passwords_range['value']}",
                length=16,
                special=True,
                override_special=args["servers"][server_passwords_range["value"]]["name"],
                opts = pulumi.ResourceOptions(parent=self)))

        # Example of iterating a map of objects
        zone_passwords: dict[str, random.RandomPassword] = {}
        for zone_passwords_range in [{"key": k, "value": v} for [k, v] in sorted((args["deploymentZones"]).items())]:
            zone_passwords[zone_passwords_range['key']] = random.RandomPassword(f"{name}-zonePasswords-{zone_passwords_range['key']}",
                length=16,
                special=True,
                override_special=zone_passwords_range["value"]["zone"],
                opts = pulumi.ResourceOptions(parent=self))

        simple_component = SimpleComponent(f"{name}-simpleComponent", opts = pulumi.ResourceOptions(parent=self))

        self.result = password.result
        self.register_outputs({
            'result': password.result
        })