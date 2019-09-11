import json
import setuptools

kwargs = json.loads("""
{
    "name": "pulumipy",
    "version": "1.0.0",
    "description": "experiments",
    "url": "github.com:pulumi/pulumi.git",
    "long_description_content_type": "text/markdown",
    "author": "pulumi",
    "project_urls": {
        "Source": "github.com:pulumi/pulumi.git"
    },
    "package_dir": {
        "": "src"
    },
    "packages": [
        "pulumipy",
        "pulumipy._jsii"
    ],
    "package_data": {
        "pulumipy._jsii": [
            "experiments@1.0.0.jsii.tgz"
        ],
        "pulumipy": [
            "py.typed"
        ]
    },
    "python_requires": ">=3.6",
    "install_requires": [
        "jsii~=0.13.3",
        "publication>=0.0.3"
    ]
}
""")

with open('README.md') as fp:
    kwargs['long_description'] = fp.read()


setuptools.setup(**kwargs)
