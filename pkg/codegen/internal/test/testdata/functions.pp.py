import pulumi
import base64

encoded = base64.b64encode("haha business".encode()).decode()
joined = "-".join([
    "haha",
    "business",
])
