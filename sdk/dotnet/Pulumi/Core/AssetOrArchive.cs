// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Base class of <see cref="Asset"/>s and <see cref="Archive"/>s.
    /// </summary>
    public abstract class AssetOrArchive
    {
        private protected AssetOrArchive()
        {
        }

        internal abstract (string sigKey, string propName, object value) GetSerializationData();
    }
}
