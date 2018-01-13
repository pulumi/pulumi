# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

"""
Resource-related runtime functions.  These are not designed for external use.
"""

def register_resource(res, typ, name, custom, props, opts): # pylint: disable=unused-argument
    """
    Registers a new resource object with a given type and name.  This call is synchronous while the resource is
    created and All properties will be initialized to real property values once it completes.
    """

def get_root_resource():
    """
    Returns the implicit root stack resource for all resources created in this program.
    """
    return None
