// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;

namespace Pulumi
{
    /// <summary>
    /// Asset represents a single blob of text or data that is managed as a first class entity.
    /// </summary>
    public abstract class Asset
    {
        private protected Asset()
        {
        }
    }

    /// <summary>
    /// FileAsset is a kind of asset produced from a given path to a file on the local filesystem.
    /// </summary>
    public sealed class FileAsset : Asset
    {
        /// <summary>
        /// The path to the asset file.
        /// </summary>
        internal string Path { get; }

        public FileAsset(string path)
            => this.Path = path ?? throw new ArgumentNullException(nameof(path));
    }


    /// <summary>
    /// StringAsset is a kind of asset produced from an in-memory UTF8-encoded string.
    /// </summary>
    public sealed class StringAsset : Asset
    {
        /// <summary>
        /// The string contents.
        /// </summary>
        internal string Text { get; }

        public StringAsset(string text)
            => this.Text = text ?? throw new ArgumentNullException(nameof(text));
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
        internal string Uri { get; }

        public RemoteAsset(string uri)
            => this.Uri = uri ?? throw new ArgumentNullException(nameof(uri));
    }
}
