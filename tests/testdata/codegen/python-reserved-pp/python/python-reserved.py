import pulumi
import pulumi_lambda as lambda_

assert_ = lambda_.lambda_.Lambda("assert", lambda_="dns")
pulumi.export("global", assert_.lambda_)
