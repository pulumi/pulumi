# Copyright 2025, Pulumi Corporation.  All rights reserved.
#
# This test has a transform that raises an exception while another transform is
# executing.
#
# The register_resource call for the `fail` resource will return an error to the
# Python program, which should lead to the shutdown of the Pulumi program.
# However at this point the transform `wait` will be running, and we need to
# ensure we are not stuck on this.
#
# The shim for Python programs runs the user program in the eventloop:
#
#    exec = ThreadPoolExecutor()
#    loop.set_default_executor(exec)
#    coro = pulumi.runtime.run_in_stack(program)
#    loop.run_until_complete(coro)
#
# When the user's program raises an exception in the `fail` transform, it gets
# raised from `run_until_complete` and the loop stops processing any tasks.
# Meanwhile the register_resource call for the `wait` resource is running in the
# thread pool, but is waiting for an async task of the event loop (the `wait`
# callback) to complete. This will never happen since the loop is now stopped.
# The way out of this is to shutdown the callbacks server, which will cancel the
# transform call, which in turn unblocks the register_resource call, and thus
# completes the threadpool's outstanding work. The program can then terminate.
import asyncio
import pulumi

from echo import Echo


ready = False


async def wait(args):
    global ready
    ready = True
    await asyncio.sleep(180)
    return pulumi.ResourceTransformResult(props=args.props, opts=args.opts)


async def fail(args):
    while not ready:
        await asyncio.sleep(1)
    raise Exception("delay failed")


Echo("wait", echo="wait", opts=pulumi.ResourceOptions(transforms=[wait]))

Echo("fail", echo="fail", opts=pulumi.ResourceOptions(transforms=[fail]))
