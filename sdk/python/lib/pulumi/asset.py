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
from typing import Dict, Union

from .runtime import known_types


@known_types.asset
class Asset:
    """
    Asset represents a single blob of text or data that is managed as a first
    class entity.
    """
    pass


@known_types.file_asset
class FileAsset(Asset):
    path: str

    """
    A FileAsset is a kind of asset produced from a given path to a file on
    the local filesysetm.
    """
    def __init__(self, path: str) -> None:
        if not isinstance(path, str):
            raise TypeError("FileAsset path must be a string")
        self.path = path


@known_types.string_asset
class StringAsset(Asset):
    text: str

    """
    A StringAsset is a kind of asset produced from an in-memory UTF-8 encoded string.
    """
    def __init__(self, text: str) -> None:
        if not isinstance(text, str):
            raise TypeError("StringAsset data must be a string")
        self.text = text


@known_types.remote_asset
class RemoteAsset(Asset):
    uri: str

    """
    A RemoteAsset is a kind of asset produced from a given URI string. The URI's scheme
    dictates the protocol for fetching contents: "file://" specifies a local file, "http://"
    and "https://" specify HTTP and HTTPS, respectively. Note that specific providers may recognize
    alternative schemes; this is merely the base-most set that all providers support.
    """
    def __init__(self, uri: str) -> None:
        if not isinstance(uri, str):
            raise TypeError("RemoteAsset URI must be a string")
        self.uri = uri


@known_types.archive
class Archive:
    """
    Asset represents a collection of named assets.
    """
    pass


@known_types.asset_archive
class AssetArchive(Archive):
    assets: Dict[str, Union[Asset, Archive]]

    """
    An AssetArchive is an archive created from an in-memory collection of named assets or other archives.
    """
    def __init__(self, assets: Dict[str, Union[Asset, Archive]]) -> None:
        if not isinstance(assets, dict):
            raise TypeError("AssetArchive assets must be a dictionary")
        for k, v in assets.items():
            if not isinstance(k, str):
                raise TypeError("AssetArchive keys must be strings")
            if not isinstance(v, Asset) and not isinstance(v, Archive):
                raise TypeError("AssetArchive assets must contain only Assets or Archives")
        self.assets = assets


@known_types.file_archive
class FileArchive(Archive):
    path: str

    """
    A FileArchive is a file-based archive, or collection of file-based assets.  This can be
    a raw directory or a single archive file in one of the supported formats (.tar, .tar.gz, or .zip).
    """
    def __init__(self, path: str) -> None:
        if not isinstance(path, str):
            raise TypeError("FileArchive path must be a string")
        self.path = path


@known_types.remote_archive
class RemoteArchive(Archive):
    uri: str

    """
    A RemoteArchive is a file-based archive fetched from a remote location.  The URI's scheme dictates
    the protocol for fetching contents: "file://" specifies a local file, "http://" and "https://"
    specify HTTP and HTTPS, respectively, and specific providers may recognize custom schemes.
    """
    def __init__(self, uri: str) -> None:
        if not isinstance(uri, str):
            raise TypeError("RemoteArchive URI must be a string")
        self.uri = uri
