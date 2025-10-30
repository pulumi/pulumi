package report

import report "github.com/pulumi/pulumi/sdk/v3/pkg/codegen/report"

type GenerateProgramFn = report.GenerateProgramFn

type Reporter = report.Reporter

type Summary = report.Summary

type Stats = report.Stats

type Language = report.Language

type File = report.File

var ExportTargetDir = report.ExportTargetDir

func New(name, version string) Reporter {
	return report.New(name, version)
}

func WrapGen(reporter Reporter, title, language string, files []*syntax.File, f GenerateProgramFn) GenerateProgramFn {
	return report.WrapGen(reporter, title, language, files, f)
}

