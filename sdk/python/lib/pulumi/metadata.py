# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

from runtime import SETTINGS

def get_project():
    """
    Returns the current project name.
    """
    return SETTINGS.project

def get_stack():
    """
    Returns the current stack name.
    """
    return SETTINGS.stack
