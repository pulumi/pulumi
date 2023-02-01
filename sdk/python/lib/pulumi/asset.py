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

"""
Assets are the Pulumi notion of data blobs that can be passed to resources.
"""
from os import PathLike, fspath
from typing import Dict, Union


class Asset:
    """
    Asset represents a single blob of text or data that is managed as a first
    class entity.
    """


class FileAsset(Asset):
    """
    A FileAsset is a kind of asset produced from a given path to a file on
    the local filesystem.
    """

    path: str

    def __init__(self, path: Union[str, PathLike]) -> None:
        if not isinstance(path, (str, PathLike)):
            raise TypeError("FileAsset path must be a string or os.PathLike")
        self.path = fspath(path)


class StringAsset(Asset):
    """
    A StringAsset is a kind of asset produced from an in-memory UTF-8 encoded string.
    """

    text: str

    def __init__(self, text: str) -> None:
        if not isinstance(text, str):
            raise TypeError("StringAsset data must be a string")
        self.text = text


class RemoteAsset(Asset):
    """
    A RemoteAsset is a kind of asset produced from a given URI string. The URI's scheme
    dictates the protocol for fetching contents: "file://" specifies a local file, "http://"
    and "https://" specify HTTP and HTTPS, respectively. Note that specific providers may recognize
    alternative schemes; this is merely the base-most set that all providers support.
    """

    uri: str

    def __init__(self, uri: str) -> None:
        if not isinstance(uri, str):
            raise TypeError("RemoteAsset URI must be a string")
        self.uri = uri


class Archive:
    """
    Archive represents a collection of named assets.
    """


class AssetArchive(Archive):
    """
    An AssetArchive is an archive created from an in-memory collection of named assets or other archives.
    """

    assets: Dict[str, Union[Asset, Archive]]

    def __init__(self, assets: Dict[str, Union[Asset, Archive]]) -> None:
        if not isinstance(assets, dict):
            raise TypeError("AssetArchive assets must be a dictionary")
        for k, v in assets.items():
            if not isinstance(k, str):
                raise TypeError("AssetArchive keys must be strings")
            if not isinstance(v, Asset) and not isinstance(v, Archive):
                raise TypeError(
                    "AssetArchive assets must contain only Assets or Archives"
                )
        self.assets = assets


class FileArchive(Archive):
    """
    A FileArchive is a file-based archive, or collection of file-based assets.  This can be
    a raw directory or a single archive file in one of the supported formats (.tar, .tar.gz, or .zip).
    """

    path: str

    def __init__(self, path: str) -> None:
        if not isinstance(path, str):
            raise TypeError("FileArchive path must be a string")
        self.path = path


class RemoteArchive(Archive):
    """
    A RemoteArchive is a file-based archive fetched from a remote location.  The URI's scheme dictates
    the protocol for fetching contents: "file://" specifies a local file, "http://" and "https://"
    specify HTTP and HTTPS, respectively, and specific providers may recognize custom schemes.
    """

    uri: str

    def __init__(self, uri: str) -> None:
        if not isinstance(uri, str):
            raise TypeError("RemoteArchive URI must be a string")
        self.uri = uri
