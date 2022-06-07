// Copyright 2016-2021, Pulumi Corporation

namespace Pulumi.Automation
{
    public enum OperationType
    {
        Same,
        Create,
        Update,
        Delete,
        Replace,
        CreateReplacement,
        DeleteReplaced,
        Read,
        ReadReplacement,
        Refresh,
        ReadDiscard,
        DiscardReplaced,
        RemovePendingReplace,
        Import,
        ImportReplacement,
    }
}
