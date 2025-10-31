package fuzzing

import fuzzing "github.com/pulumi/pulumi/sdk/v3/pkg/engine/lifecycletest/fuzzing"

// A ProgramSpec specifies a Pulumi program whose execution will be mocked in a lifecycle test in order to register
// resources.
type ProgramSpec = fuzzing.ProgramSpec

// The type of tags that may be added to resources in a ProgramSpec.
type ProgramResourceTag = fuzzing.ProgramResourceTag

// A set of options for configuring the generation of a ProgramSpec.
type ProgramSpecOptions = fuzzing.ProgramSpecOptions

// The set of actions that may be taken on resources in a ProgramSpec.
type ProgramSpecAction = fuzzing.ProgramSpecAction

const NewlyPrependedProgramResource = fuzzing.NewlyPrependedProgramResource

const DroppedProgramResource = fuzzing.DroppedProgramResource

const NewlyInsertedProgramResource = fuzzing.NewlyInsertedProgramResource

const UpdatedProgramResource = fuzzing.UpdatedProgramResource

const CopiedProgramResource = fuzzing.CopiedProgramResource

const NewlyAppendedProgramResource = fuzzing.NewlyAppendedProgramResource

const ProgramSpecDelete = fuzzing.ProgramSpecDelete

const ProgramSpecInsert = fuzzing.ProgramSpecInsert

const ProgramSpecUpdate = fuzzing.ProgramSpecUpdate

const ProgramSpecCopy = fuzzing.ProgramSpecCopy

// Given a SnapshotSpec and a set of options, returns a rapid.Generator that will produce ProgramSpecs that operate upon
// the specified snapshot.
func GeneratedProgramSpec(ss *SnapshotSpec, sso StackSpecOptions, pso ProgramSpecOptions) *interface{} {
	return fuzzing.GeneratedProgramSpec(ss, sso, pso)
}

