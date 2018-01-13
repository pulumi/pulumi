# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

"""The Resource module, containing all resource-related definitions."""

from pulumi import runtime

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
            opts.parent = runtime.get_root_resource() # pylint: disable=assignment-from-none

        # Now register the resource.  If we are actually performing a deployment, this resource's properties
        # will be resolved to real values.  If we are only doing a dry-run preview, on the other hand, they will
        # resolve to special Preview sentinel values to indicate the value isn't yet available.
        runtime.register_resource(self, t, name, custom, props, opts)

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

def export(name, value):
    """
    Exports a named stack output.
    """
    # TODO
