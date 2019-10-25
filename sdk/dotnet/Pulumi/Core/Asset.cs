// Copyright 2016-2019, Pulumi Corporation

using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Asset represents a single blob of text or data that is managed as a first class entity.
    /// </summary>
    public abstract class Asset : AssetOrArchive
    {
        private protected Asset(string propName, object value)
            : base(Constants.SpecialAssetSig, propName, value)
        {
        }
    }

    /// <summary>
    /// FileAsset is a kind of asset produced from a given path to a file on the local filesystem.
    /// </summary>
    public sealed class FileAsset : Asset
    {
        public FileAsset(string path) : base(Constants.AssetOrArchivePathName, path)
        {
        }
    }


    /// <summary>
    /// StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
    /// </summary>
    public sealed class StringAsset : Asset
    {
        public StringAsset(string text) : base(Constants.AssetTextName, text)
        {
        }
    }

    /// <summary>
    /// RemoteAsset is a kind of asset produced from a given URI string.  The URI's scheme dictates
    /// the protocol for fetching contents: <c>file://</c> specifies a local file, <c>http://</c>
    /// and <c>https://</c> specify HTTP and HTTPS, respectively.  Note that specific providers may
    /// recognize alternative schemes; this is merely the base-most set that all providers support.
    /// </summary>
    public sealed class RemoteAsset : Asset
    {
        public RemoteAsset(string uri) : base(Constants.AssetOrArchiveUriName, uri)
        {
        }
    }
}
