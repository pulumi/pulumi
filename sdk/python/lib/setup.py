# Copyright 2016-2022, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""The Pulumi Python SDK."""

from setuptools import find_packages, setup

VERSION = "3.0.0"


def readme():
    try:
        with open('README.md', encoding='utf-8') as f:
            return f.read()
    except FileNotFoundError:
        return "Pulumi's Python SDK - Development Version"


setup(name='pulumi',
      version=VERSION,
      description='Pulumi\'s Python SDK',
      long_description=readme(),
      long_description_content_type='text/markdown',
      url='https://github.com/pulumi/pulumi',
      license='Apache 2.0',
      packages=find_packages(exclude=("test*",)),
      package_data={
          'pulumi': [
              'py.typed'
          ]
      },
      python_requires='>=3.7',
      # Keep this list in sync with Pipfile
      install_requires=[
          'protobuf~=4.21',
          'grpcio==1.56.2',
          'dill~=0.3',
          'six~=1.12',
          'semver~=2.13',
          'pyyaml~=6.0'
      ],
      zip_safe=False)
