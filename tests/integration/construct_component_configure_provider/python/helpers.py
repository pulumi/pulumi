import asyncio
import pulumi


def unknownIfDryRun(value):
    if pulumi.runtime.is_dry_run():
        return pulumi.Output(resources=set(), future=fut(None), is_known=fut(False))
    return pulumi.Output.from_input(value)


def fut(x):
    f = asyncio.Future()
    f.set_result(x)
    return f
