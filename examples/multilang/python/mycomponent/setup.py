# Copyright 2016-2020, Pulumi Corporation.
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

import errno
from setuptools import setup, find_packages
from setuptools.command.install import install
from setuptools.command.develop import develop
from subprocess import check_call

def npm_install():
    # Using `yarn` here because this package is designed specifically to be used in our tests, and we need to be able to
    # `yarn link` to test at this layer.
    check_call(['yarn', 'install'])

def npm_link_pulumi():
    # Using `yarn` here because this package is designed specifically to be used in our tests, and we need to be able to
    # `yarn link` to test at this layer.
    check_call(['yarn', 'link', '@pulumi/pulumi'])

class InstallNPMPackageCommand(install):
    def run(self):
        install.run(self)
        npm_install()

class DevelopNPMPackageCommand(develop):
    def run(self):
        develop.run(self)
        npm_install()
        npm_link_pulumi()

def readme():
    with open('README.md', encoding='utf-8') as f:
        return f.read()

setup(
    name='pulumi_mycomponent',
    version='0.0.1',
    description='MyComponent',
    long_description=readme(),
    long_description_content_type='text/markdown',
    cmdclass={
        'install': InstallNPMPackageCommand,
        'develop': DevelopNPMPackageCommand,
    },
    url='https://github.com/pulumi/pulumi',
    license='Apache 2.0',
    packages=find_packages(),
    zip_safe=False)
