# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

"""
Runtime settings and configuration.
"""

import grpc
from proto import engine_pb2_grpc, resource_pb2_grpc
from ..errors import RunError

class Settings(object):
    """
    A bag of properties for configuring the Pulumi Python language runtime.
    """
    def __init__(self, monitor=None, engine=None, project=None, stack=None, parallel=None, dry_run=None):
        # Save the metadata information.
        self.project = project
        self.stack = stack
        self.parallel = parallel
        self.dry_run = dry_run

        # Actually connect to the monitor/engine over gRPC.
        if monitor:
            self.monitor = resource_pb2_grpc.ResourceMonitorStub(grpc.insecure_channel(monitor))
        else:
            self.monitor = None
        if engine:
            self.engine = engine_pb2_grpc.EngineStub(grpc.insecure_channel(engine))
        else:
            self.engine = None

# default to "empty" settings.
SETTINGS = Settings()

def configure(settings):
    """
    Configure sets the current ambient settings bag to the one given.
    """
    if not settings or not isinstance(settings, Settings):
        raise TypeError('Settings is expected to be non-None and of type Settings')
    global SETTINGS # pylint: disable=global-statement
    SETTINGS = settings

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

def get_monitor():
    """
    Returns the current resource monitoring service client for RPC communications.
    """
    monitor = SETTINGS.monitor
    if not monitor:
        raise RunError('Pulumi program not connected to the engine -- are you running with the `pulumi` CLI?')
    return monitor

ROOT = None

def get_root_resource():
    """
    Returns the implicit root stack resource for all resources created in this program.
    """
    global ROOT
    return ROOT

def set_root_resource(root):
    """
    Sets the current root stack resource for all resources subsequently to be created in this program.
    """
    global ROOT
    ROOT = root
