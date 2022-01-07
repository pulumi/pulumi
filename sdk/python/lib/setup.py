# Copyright 2016-2018, Pulumi Corporation.
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

from setuptools import setup, find_packages


def readme():
    with open('README.md', encoding='utf-8') as f:
        return f.read()


setup(name='pulumi',
      version='${VERSION}',
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
      # Keep this list in sync with Pipfile
      install_requires=[
          'protobuf>=3.6.0',
          'dill>=0.3.0',
          'grpcio>=1.33.2',
          'six>=1.12.0',
          'semver>=2.8.1',
          'pyyaml>=5.3.1'
      ],
      zip_safe=False)
