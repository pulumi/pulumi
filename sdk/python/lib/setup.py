# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

"""The Pulumi Python SDK."""

from setuptools import setup

setup(name='pulumi',
      version='${VERSION}',
      description='Pulumi\'s Python SDK',
      url='https://github.com/pulumi/pulumi',
      packages=['pulumi', 'pulumi.runtime'],
      zip_safe=False)
