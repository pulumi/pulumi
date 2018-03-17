# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import runtime

def get_project():
    """
    Returns the current project name.
    """
    return runtime.get_project()

def get_stack():
    """
    Returns the current stack name.
    """
    return runtime.get_stack()
