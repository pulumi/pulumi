#!/usr/bin/env python
# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import argparse
import asyncio
from typing import Optional
import logging
import os
import sys
import traceback
import runpy
from concurrent.futures import ThreadPoolExecutor

# The user might not have installed Pulumi yet in their environment - provide a high-quality error message in that case.
try:
    import pulumi
    import pulumi.runtime
except ImportError:
    # For whatever reason, sys.stderr.write is not picked up by the engine as a message, but 'print' is. The Python
    # langhost automatically flushes stdout and stderr on shutdown, so we don't need to do it here - just trust that
    # Python does the sane thing when printing to stderr.
    print(traceback.format_exc(), file=sys.stderr)
    print("""
It looks like the Pulumi SDK has not been installed. Have you run pip install?
If you are running in a virtualenv, you must run pip install -r requirements.txt from inside the virtualenv.""", file=sys.stderr)
    sys.exit(1)

# use exit code 32 to signal to the language host that an error message was displayed to the user
PYTHON_PROCESS_EXITED_AFTER_SHOWING_USER_ACTIONABLE_MESSAGE_CODE = 32

def get_abs_module_path(mod_path):
    path, ext = os.path.splitext(mod_path)
    if not ext:
        path = os.path.join(path, '__main__')
    return os.path.abspath(path)


def _get_user_stacktrace(user_program_abspath: str) -> str:
    '''grabs the current stacktrace and truncates it to show the only stacks pertaining to a user's program'''
    tb = traceback.extract_tb(sys.exc_info()[2])

    for frame_index, frame in enumerate(tb):
        # loop over stack frames until we reach the main program
        # then return the traceback truncated to the user's code
        cur_module = frame[0]
        if get_abs_module_path(user_program_abspath) == get_abs_module_path(cur_module):
            # we have detected the start of a user's stack trace
            remaining_frames = len(tb)-frame_index

            # include remaining frames from the bottom by negating
            return traceback.format_exc(limit=-remaining_frames)

    # we did not detect a __main__ program, return normal traceback
    return traceback.format_exc()

def _set_default_executor(loop, parallelism: Optional[int]):
    '''configure this event loop to respect the settings provided.'''
    if parallelism is None:
        return
    parallelism = max(parallelism, 1)
    exec = ThreadPoolExecutor(max_workers=parallelism)
    loop.set_default_executor(exec)

if __name__ == "__main__":
    # Parse the arguments, program name, and optional arguments.
    ap = argparse.ArgumentParser(description='Execute a Pulumi Python program')
    ap.add_argument('--project', help='Set the project name')
    ap.add_argument('--stack', help='Set the stack name')
    ap.add_argument('--parallel', help='Run P resource operations in parallel (default=none)')
    ap.add_argument('--dry_run', help='Simulate resource changes, but without making them')
    ap.add_argument('--pwd', help='Change the working directory before running the program')
    ap.add_argument('--monitor', help='An RPC address for the resource monitor to connect to')
    ap.add_argument('--engine', help='An RPC address for the engine to connect to')
    ap.add_argument('--tracing', help='A Zipkin-compatible endpoint to send tracing data to')
    ap.add_argument('--organization', help='Set the organization name')
    ap.add_argument('PROGRAM', help='The Python program to run')
    ap.add_argument('ARGS', help='Arguments to pass to the program', nargs='*')
    args = ap.parse_args()

    # If any config variables are present, parse and set them, so subsequent accesses are fast.
    config_env = pulumi.runtime.get_config_env()
    if hasattr(pulumi.runtime, "get_config_secret_keys_env") and hasattr(pulumi.runtime, "set_all_config"):
        # If the pulumi SDK has `get_config_secret_keys_env` and `set_all_config`, use them
        # to set the config and secret keys.
        config_secret_keys_env = pulumi.runtime.get_config_secret_keys_env()
        pulumi.runtime.set_all_config(config_env, config_secret_keys_env)
    else:
        # Otherwise, fallback to setting individual config values.
        for k, v in config_env.items():
            pulumi.runtime.set_config(k, v)

    # Configure the runtime so that the user program hooks up to Pulumi as appropriate.
    # New versions of pulumi python support setting organization, old versions do not
    try:
        settings = pulumi.runtime.Settings(
            monitor=args.monitor,
            engine=args.engine,
            project=args.project,
            stack=args.stack,
            parallel=int(args.parallel),
            dry_run=args.dry_run == "true",
            organization=args.organization,
        )
    except TypeError:
        settings = pulumi.runtime.Settings(
            monitor=args.monitor,
            engine=args.engine,
            project=args.project,
            stack=args.stack,
            parallel=int(args.parallel),
            dry_run=args.dry_run == "true"
        )

    pulumi.runtime.configure(settings)

    # Finally, swap in the args, chdir if needed, and run the program as if it had been executed directly.
    sys.argv = [args.PROGRAM] + args.ARGS
    if args.pwd is not None:
        os.chdir(args.pwd)

    successful = False

    try:
        # The docs for get_running_loop are somewhat misleading because they state:
        # This function can only be called from a coroutine or a callback. However, if the function is
        # called from outside a coroutine or callback (the standard case when running `pulumi up`), the function
        # raises a RuntimeError as expected and falls through to the exception clause below.
        loop = asyncio.get_running_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    
    # Configure the event loop to respect the parallelism value provided as input.
    _set_default_executor(loop, settings.parallel)

    # We are (unfortunately) suppressing the log output of asyncio to avoid showing to users some of the bad things we
    # do in our programming model.
    #
    # Fundamentally, Pulumi is a way for users to build asynchronous dataflow graphs that, as their deployments
    # progress, resolve naturally and eventually result in the complete resolution of the graph. If one node in the
    # graph fails (i.e. a resource fails to create, there's an exception in an apply, etc.), part of the graph remains
    # unevaluated at the time that we exit.
    #
    # asyncio abhors this. It gets very upset if the process terminates without having observed every future that we
    # have resolved. If we are terminating abnormally, it is highly likely that we are not going to observe every single
    # future that we have created. Furthermore, it's *harmless* to do this - asyncio logs errors because it thinks it
    # needs to tell users that they're doing bad things (which, to their credit, they are), but we are doing this
    # deliberately.
    #
    # In order to paper over this for our users, we simply turn off the logger for asyncio. Users won't see any asyncio
    # error messages, but if they stick to the Pulumi programming model, they wouldn't be seeing any anyway.
    logging.getLogger("asyncio").setLevel(logging.CRITICAL)
    exit_code = 1
    try:
        # record the location of the user's program to return user tracebacks
        user_program_abspath = os.path.abspath(args.PROGRAM)
        def run():
            try:
                runpy.run_path(args.PROGRAM, run_name='__main__')
            except ImportError as e:
                def fix_module_file(m: str) -> str:
                    # Work around python 11 reporting "<frozen runpy>" rather
                    # than runpy.__file__ in the traceback.
                    return runpy.__file__ if m == "<frozen runpy>" else m

                # detect if the main pulumi python program does not exist
                stack_modules = [fix_module_file(f.filename) for f in traceback.extract_tb(e.__traceback__)]
                unique_modules = set(module for module in stack_modules)
                last_module_name = stack_modules[-1]

                # we identify a missing program error if
                # 1. the only modules in the stack trace are
                #   - `pulumi-language-python-exec`
                #   - `runpy`
                # 2. the last function in the stack trace is in the `runpy` module
                if unique_modules == {
                            __file__, # the language runtime itself
                            runpy.__file__,
                        } and last_module_name == runpy.__file__ :
                    # this error will only be hit when the user provides a directory
                    # the engine has a check to determine if the `main` file exists and will fail early

                    # if a language runtime receives a directory, it's the language's responsibility to determine
                    # whether the provided directory has a pulumi program
                    pulumi.log.error(f"unable to find main python program `__main__.py` in `{user_program_abspath}`")
                    sys.exit(PYTHON_PROCESS_EXITED_AFTER_SHOWING_USER_ACTIONABLE_MESSAGE_CODE)
                else:
                    raise e

        coro = pulumi.runtime.run_in_stack(run)
        loop.run_until_complete(coro)
        exit_code = 0
    except pulumi.RunError as e:
        pulumi.log.error(str(e))
    except Exception:
        error_msg = "Program failed with an unhandled exception:\n" + _get_user_stacktrace(user_program_abspath)
        pulumi.log.error(error_msg)
        exit_code = PYTHON_PROCESS_EXITED_AFTER_SHOWING_USER_ACTIONABLE_MESSAGE_CODE
    finally:
        loop.close()
        sys.stdout.flush()
        sys.stderr.flush()

    sys.exit(exit_code)
