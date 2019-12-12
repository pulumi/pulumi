// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Base class of <see cref="Asset"/>s and <see cref="Archive"/>s.
    /// </summary>
    public abstract class AssetOrArchive
    {
        internal string SigKey { get; }
        internal string PropName { get; }
        internal object Value { get; }

        private protected AssetOrArchive(string sigKey, string propName, object value)
        {
            SigKey = sigKey ?? throw new System.ArgumentNullException(nameof(sigKey));
            PropName = propName ?? throw new System.ArgumentNullException(nameof(propName));
            Value = value ?? throw new System.ArgumentNullException(nameof(value));
        }
    }
}
