from setuptools import setup, find_packages

setup(name='pulumi_foo',
      python_requires='>=3.8',
      version="1.0.0",
      long_description="A description",
      long_description_content_type='text/markdown',
      packages=find_packages(),
      package_data={
          'pulumi_foo': [
              'pulumi-plugin.json',
              'py.typed',
          ]
      },
      install_requires=[],
      zip_safe=False)
