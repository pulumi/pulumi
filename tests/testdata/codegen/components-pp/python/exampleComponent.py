import pulumi
from pulumi import Input
from simpleComponent import SimpleComponent
from typing import Optional, Dict, TypedDict, Any
import pulumi_random as random

class DeploymentZones(TypedDict, total=False):
    zone: Input[str]

class GithubApp(TypedDict, total=False):
    id: Input[str]
    keyBase64: Input[str]
    webhookSecret: Input[str]

class Servers(TypedDict, total=False):
    name: Input[str]

class ExampleComponentArgs(TypedDict, total=False):
    input: Input[str]
    cidrBlocks: Input[Dict[str, str]]
    githubApp: Input[GithubApp]
    servers: Input[list(Servers)]
    deploymentZones: Input[Dict[str, DeploymentZones]]
    ipAddress: Input[list[int]]

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
        server_passwords = []
        for range in [{"value": i} for i in range(0, len(args["servers"]))]:
            server_passwords.append(random.RandomPassword(f"{name}-serverPasswords-{range['value']}",
                length=16,
                special=True,
                override_special=args["servers"][range["value"]]["name"],
                opts = pulumi.ResourceOptions(parent=self)))

        # Example of iterating a map of objects
        zone_passwords = []
        for range in [{"key": k, "value": v} for [k, v] in enumerate(args["deploymentZones"])]:
            zone_passwords.append(random.RandomPassword(f"{name}-zonePasswords-{range['key']}",
                length=16,
                special=True,
                override_special=range["value"]["zone"],
                opts = pulumi.ResourceOptions(parent=self)))

        simple_component = SimpleComponent(f"{name}-simpleComponent", opts = pulumi.ResourceOptions(parent=self))

        self.result = password.result
        self.register_outputs({
            'result': password.result
        })