# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

"""The Resource module, containing all resource-related definitions."""

from runtime.resource import register_resource, register_resource_outputs
from runtime.settings import get_root_resource

class Resource(object):
    """
    Resource represents a class whose CRUD operations are implemented by a provider plugin.
    """
    def __init__(self, t, name, custom, props=None, opts=None):
        if not t:
            raise TypeError('Missing resource type argument')
        if not isinstance(t, basestring):
            raise TypeError('Expected resource type to be a string')
        if not name:
            raise TypeError('Missing resource name argument (for URN creation)')
        if not isinstance(name, basestring):
            raise TypeError('Expected resource name to be a string')

        # Properties and options can be missing; simply, initialize to empty dictionaries.
        if props:
            if not isinstance(props, dict):
                raise TypeError('Expected resource properties to be a dictionary')
        elif not props:
            props = dict()
        if opts:
            if not isinstance(opts, ResourceOptions):
                raise TypeError('Expected resource options to be a ResourceOptions instance')
        if not opts:
            opts = ResourceOptions()

        # Default the parent if there is none.
        if opts.parent is None:
            opts.parent = get_root_resource()

        # Now register the resource.  If we are actually performing a deployment, this resource's properties
        # will be resolved to real values.  If we are only doing a dry-run preview, on the other hand, they will
        # resolve to special Preview sentinel values to indicate the value isn't yet available.
        register_resource(self, t, name, custom, props, opts)

    def set_outputs(self, outputs):
        """
        Sets output properties after a registration has completed.
        """
        # By default, do nothing.  If subclasses wish to support provider outputs, they must override this.

class ResourceOptions(object):
    """
    ResourceOptions is a bag of optional settings that control a resource's behavior.
    """
    def __init__(self, parent=None, depends_on=None, protect=None):
        self.parent = parent
        self.depends_on = depends_on
        self.protect = protect

class CustomResource(Resource):
    """
    CustomResource is a resource whose CRUD operations are managed by performing external operations on some
    physical entity.  Pulumi understands how to diff and perform partial updates ot them, and these CRUD operations
    are implemented in a dynamically loaded plugin for the defining package.
    """
    def __init__(self, t, name, props=None, opts=None):
        Resource.__init__(self, t, name, True, props, opts)

class ComponentResource(Resource):
    """
    ComponentResource is a resource that aggregates one or more other child resources into a higher level
    abstraction.  The component itself is a resource, but does not require custom CRUD operations for provisioning.
    """
    def __init__(self, t, name, props=None, opts=None):
        Resource.__init__(self, t, name, False, props, opts)

    def register_outputs(self, outputs):
        """
        Register synthetic outputs that a component has initialized, usually by allocating other child
        sub-resources and propagating their resulting property values.
        """
        if outputs:
            register_resource_outputs(self, outputs)

def export(name, value):
    """
    Exports a named stack output.
    """
    stack = get_root_resource()
    if stack is not None:
        stack.export(name, value)
