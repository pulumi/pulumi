// Copyright 2016-2018, Pulumi Corporation

#nullable enable

namespace Pulumi
{
    /// <summary>
    /// Base class of <see cref="Asset"/>s and <see cref="Archive"/>s.
    /// </summary>
    public abstract class AssetOrArchive
    {
        internal abstract (string sigKey, string propName, object value) GetSerializationData();
    }
}
