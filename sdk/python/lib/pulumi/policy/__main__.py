# Copyright 2016-2020, Pulumi Corporation.  All rights reserved.

import os
import sys
import traceback
import runpy

import pulumi
import pulumi.runtime


def main():
    if len(sys.argv) != 3:
        # For whatever reason, sys.stderr.write is not picked up by the engine as a message, but 'print' is. The Python
        # langhost automatically flushes stdout and stderr on shutdown, so we don't need to do it here - just trust that
        # Python does the sane thing when printing to stderr.
        print("usage: python3 -u -m pulumi.policy <engine-address> <program>", file=sys.stderr)
        sys.exit(1)

    program = sys.argv[2]

    # If any config variables are present, parse and set them, so subsequent accesses are fast.
    config_env = pulumi.runtime.get_config_env()
    for k, v in config_env.items():
        pulumi.runtime.set_config(k, v)

    # Configure the runtime so that the user program hooks up to Pulumi as appropriate.
    if 'PULUMI_PROJECT' in os.environ and 'PULUMI_STACK' in os.environ and 'PULUMI_DRY_RUN' in os.environ:
        pulumi.runtime.configure(
            pulumi.runtime.Settings(
                project=os.environ["PULUMI_PROJECT"],
                stack=os.environ["PULUMI_STACK"],
                dry_run=os.environ["PULUMI_DRY_RUN"] == "true"
            )
        )

    successful = False

    try:
        runpy.run_path(program, run_name="__main__")
        successful = True
    except pulumi.RunError as e:
        pulumi.log.error(str(e))
    except Exception as e:
        pulumi.log.error("Program failed with an unhandled exception:")
        pulumi.log.error(traceback.format_exc())
    finally:
        sys.stdout.flush()
        sys.stderr.flush()

    exit_code = 0 if successful else 1
    sys.exit(exit_code)


main()
