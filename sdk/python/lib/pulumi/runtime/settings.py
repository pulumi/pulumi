# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

"""
Runtime settings and configuration.
"""

class Settings(object):
    """
    A bag of properties for configuring the Pulumi Python language runtime.
    """
    def __init__(self, monitor=None, engine=None, project=None, stack=None, parallel=None, dry_run=None):
        self.monitor = monitor
        self.engine = engine
        self.project = project
        self.stack = stack
        self.parallel = parallel
        self.dry_run = dry_run

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
