# Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

"""The Pulumi Python SDK."""

from setuptools import setup, find_packages

setup(name='pulumi',
      version='${VERSION}',
      description='Pulumi\'s Python SDK',
      url='https://github.com/pulumi/pulumi',
      packages=find_packages(),
      install_requires=[
          'google==2.0.1',
          'grpcio==1.9.1',
          'six==1.11.0'
      ],
      zip_safe=False)
