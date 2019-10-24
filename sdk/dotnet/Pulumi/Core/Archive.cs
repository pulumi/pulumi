// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Immutable;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// An Archive represents a collection of named assets.
    /// </summary>
    public abstract class Archive : AssetOrArchive
    {
        private protected Archive(string propName, object value)
            : base(Constants.SpecialArchiveSig, propName, value)
        {
        }
    }

    /// <summary>
    /// An AssetArchive is an archive created from an in-memory collection of named assets or other
    /// archives.
    /// </summary>
    public sealed class AssetArchive : Archive
    {
        public AssetArchive(ImmutableDictionary<string, AssetOrArchive> assets)
            : base(Constants.ArchiveAssetsName, assets)
        {
        }
    }

    /// <summary>
    /// A FileArchive is a file-based archive, or a collection of file-based assets.  This can be a
    /// raw directory or a single archive file in one of the supported formats(.tar, .tar.gz,
    /// or.zip).
    /// </summary>
    public sealed class FileArchive : Archive
    {
        public FileArchive(string path) : base(Constants.AssetOrArchivePathName, path)
        {
        }
    }

    /// <summary>
    /// A RemoteArchive is a file-based archive fetched from a remote location.  The URI's scheme
    /// dictates the protocol for fetching the archive's contents: <c>file://</c> is a local file
    /// (just like a FileArchive), <c>http://</c> and <c>https://</c> specify HTTP and HTTPS,
    /// respectively, and specific providers may recognize custom schemes.
    /// </summary>
    public sealed class RemoteArchive : Archive
    {
        public RemoteArchive(string uri) : base(Constants.AssetOrArchiveUriName, uri)
        {
        }
    }
}
