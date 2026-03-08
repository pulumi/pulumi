import pulumi
import base64

config = pulumi.Config()
input = config.require("input")
bytes = base64.b64decode(input.encode()).decode()
pulumi.export("data", bytes)
pulumi.export("roundtrip", base64.b64encode(bytes.encode()).decode())
