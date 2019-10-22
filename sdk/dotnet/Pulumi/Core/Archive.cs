// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using System.Collections.Immutable;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// An Archive represents a collection of named assets.
    /// </summary>
    public abstract class Archive : AssetOrArchive
    {
        private protected Archive()
        {
        }

        internal override (string sigKey, string propName, object value) GetSerializationData()
        {
            var (propName, value) = GetSerializationDataWorker();
            return (Constants.SpecialArchiveSig, propName, value);
        }

        internal abstract (string propName, object value) GetSerializationDataWorker();
    }

    /// <summary>
    /// An AssetArchive is an archive created from an in-memory collection of named assets or other
    /// archives.
    /// </summary>
    public sealed class AssetArchive : Archive
    {
        /// <summary>
        /// A map of names to assets.
        /// </summary>
        private readonly ImmutableDictionary<string, AssetOrArchive> _assets;

        public AssetArchive(ImmutableDictionary<string, AssetOrArchive> assets)
                => _assets = assets ?? throw new ArgumentNullException(nameof(assets));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("assets", _assets);
    }

    /// <summary>
    /// A FileArchive is a file-based archive, or a collection of file-based assets.  This can be a
    /// raw directory or a single archive file in one of the supported formats(.tar, .tar.gz,
    /// or.zip).
    /// </summary>
    public sealed class FileArchive : Archive
    {
        /// <summary>
        /// The path to the asset file.
        /// </summary>
        private readonly string _path;

        public FileArchive(string path)
            => this._path = path ?? throw new ArgumentNullException(nameof(path));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("path", _path);
    }

    /// <summary>
    /// A RemoteArchive is a file-based archive fetched from a remote location.  The URI's scheme
    /// dictates the protocol for fetching the archive's contents: <c>file://</c> is a local file
    /// (just like a FileArchive), <c>http://</c> and <c>https://</c> specify HTTP and HTTPS,
    /// respectively, and specific providers may recognize custom schemes.
    /// </summary>
    public sealed class RemoteArchive : Archive
    {
        /// <summary>
        /// The URI where the archive lives.
        /// </summary>
        private readonly string _uri;

        public RemoteArchive(string uri)
                => _uri = uri ?? throw new ArgumentNullException(nameof(uri));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("uri", _uri);
    }
}