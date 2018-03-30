# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

"""
Support for automatic stack components.
"""

from ..resource import ComponentResource
from settings import get_project, get_stack, get_root_resource, set_root_resource

def run_in_stack(func):
    """
    Run the given function inside of a new stack resource.  This ensures that any stack export calls will end
    up as output properties on the resulting stack component in the checkpoint file.  This is meant for internal
    runtime use only and is used by the Python SDK entrypoint program.
    """
    Stack(func)

class Stack(ComponentResource):
    """
    A synthetic stack component that automatically parents resources as the program runs.
    """
    def __init__(self, func):
        # Ensure we don't already have a stack registered.
        if get_root_resource() is not None:
            raise Exception('Only one root Pulumi Stack may be active at once')

        # Now invoke the registration to begin creating this resource.
        name = '%s-%s' % (get_project(), get_stack())
        super(Stack, self).__init__('pulumi:pulumi:Stack', name, None, None)

        # Invoke the function while this stack is active and then register its outputs.
        self.outputs = dict()
        set_root_resource(self)
        try:
            func()
        finally:
            self.register_outputs(self.outputs)
            # Intentionally leave this resource installed in case subsequent async work uses it.

    def output(self, name, value):
        """
        Export a stack output with a given name and value.
        """
        self.outputs[name] = value
