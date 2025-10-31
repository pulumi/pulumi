package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/backend"

// UpdateMetadata describes optional metadata about an update.
type UpdateMetadata = backend.UpdateMetadata

// UpdateResult is an enum for the result of the update.
type UpdateResult = backend.UpdateResult

// UpdateInfo describes a previous update.
type UpdateInfo = backend.UpdateInfo

const InProgressResult = backend.InProgressResult

const SucceededResult = backend.SucceededResult

const FailedResult = backend.FailedResult

// Keys we use for values put into UpdateInfo.Environment.
const GitHead = backend.GitHead

// Keys we use for values put into UpdateInfo.Environment.
const GitHeadName = backend.GitHeadName

// Keys we use for values put into UpdateInfo.Environment.
const GitDirty = backend.GitDirty

// Keys we use for values put into UpdateInfo.Environment.
const GitCommitter = backend.GitCommitter

// Keys we use for values put into UpdateInfo.Environment.
const GitCommitterEmail = backend.GitCommitterEmail

// Keys we use for values put into UpdateInfo.Environment.
const GitAuthor = backend.GitAuthor

// Keys we use for values put into UpdateInfo.Environment.
const GitAuthorEmail = backend.GitAuthorEmail

// Keys we use for values put into UpdateInfo.Environment.
const VCSRepoOwner = backend.VCSRepoOwner

// Keys we use for values put into UpdateInfo.Environment.
const VCSRepoName = backend.VCSRepoName

// Keys we use for values put into UpdateInfo.Environment.
const VCSRepoKind = backend.VCSRepoKind

// Keys we use for values put into UpdateInfo.Environment.
const VCSRepoRoot = backend.VCSRepoRoot

// Keys we use for values put into UpdateInfo.Environment.
const CISystem = backend.CISystem

// Keys we use for values put into UpdateInfo.Environment.
const CIBuildID = backend.CIBuildID

// Keys we use for values put into UpdateInfo.Environment.
const CIBuildNumer = backend.CIBuildNumer

// Keys we use for values put into UpdateInfo.Environment.
const CIBuildType = backend.CIBuildType

// Keys we use for values put into UpdateInfo.Environment.
const CIBuildURL = backend.CIBuildURL

// Keys we use for values put into UpdateInfo.Environment.
const CIPRHeadSHA = backend.CIPRHeadSHA

// Keys we use for values put into UpdateInfo.Environment.
const CIPRNumber = backend.CIPRNumber

// Keys we use for values put into UpdateInfo.Environment.
const ExecutionKind = backend.ExecutionKind

// Keys we use for values put into UpdateInfo.Environment.
const ExecutionAgent = backend.ExecutionAgent

// Keys we use for values put into UpdateInfo.Environment.
const UpdatePlan = backend.UpdatePlan

// Keys we use for values put into UpdateInfo.Environment.
const StackEnvironments = backend.StackEnvironments

