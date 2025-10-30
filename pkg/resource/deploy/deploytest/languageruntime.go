package deploytest

import deploytest "github.com/pulumi/pulumi/sdk/v3/pkg/resource/deploy/deploytest"

type LanguageRuntimeFactory = deploytest.LanguageRuntimeFactory

type ProgramFunc = deploytest.ProgramFunc

var ErrLanguageRuntimeIsClosed = deploytest.ErrLanguageRuntimeIsClosed

func NewLanguageRuntimeF(program ProgramFunc, requiredPackages ...workspace.PackageDescriptor) LanguageRuntimeFactory {
	return deploytest.NewLanguageRuntimeF(program, requiredPackages...)
}

func NewLanguageRuntime(program ProgramFunc, requiredPackages ...workspace.PackageDescriptor) plugin.LanguageRuntime {
	return deploytest.NewLanguageRuntime(program, requiredPackages...)
}

func NewLanguageRuntimeWithShutdown(program ProgramFunc, shutdown func(), requiredPackages ...workspace.PackageDescriptor) plugin.LanguageRuntime {
	return deploytest.NewLanguageRuntimeWithShutdown(program, shutdown, requiredPackages...)
}

