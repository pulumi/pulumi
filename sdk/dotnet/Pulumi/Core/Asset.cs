﻿// Copyright 2016-2019, Pulumi Corporation

#nullable enable

using System;
using Pulumi.Serialization;

namespace Pulumi
{
    /// <summary>
    /// Asset represents a single blob of text or data that is managed as a first class entity.
    /// </summary>
    public abstract class Asset : AssetOrArchive
    {
        private protected Asset()
        {
        }

        internal override (string sigKey, string propName, object value) GetSerializationData()
        {
            var (propName, value) = GetSerializationDataWorker();
            return (Constants.SpecialAssetSig, propName, value);
        }

        internal abstract (string propName, object value) GetSerializationDataWorker();
    }

    /// <summary>
    /// FileAsset is a kind of asset produced from a given path to a file on the local filesystem.
    /// </summary>
    public sealed class FileAsset : Asset
    {
        /// <summary>
        /// The path to the asset file.
        /// </summary>
        private readonly string _path;

        public FileAsset(string path)
            => _path = path ?? throw new ArgumentNullException(nameof(path));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("path", _path);
    }


    /// <summary>
    /// StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
    /// </summary>
    public sealed class StringAsset : Asset
    {
        /// <summary>
        /// The string contents.
        /// </summary>
        private readonly string _text;

        public StringAsset(string text)
            => _text = text ?? throw new ArgumentNullException(nameof(text));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("text", _text);
    }

    /// <summary>
    /// RemoteAsset is a kind of asset produced from a given URI string.  The URI's scheme dictates
    /// the protocol for fetching contents: <c>file://</c> specifies a local file, <c>http://</c>
    /// and <c>https://</c> specify HTTP and HTTPS, respectively.  Note that specific providers may
    /// recognize alternative schemes; this is merely the base-most set that all providers support.
    /// </summary>
    public sealed class RemoteAsset : Asset
    {
        /// <summary>
        /// The URI where the asset lives.
        /// </summary>
        private readonly string _uri;

        public RemoteAsset(string uri)
            => _uri = uri ?? throw new ArgumentNullException(nameof(uri));

        internal override (string propName, object value) GetSerializationDataWorker()
            => ("uri", _uri);
    }
}
