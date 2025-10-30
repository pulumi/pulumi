package newcmd

import newcmd "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/newcmd"

const BrokenTemplateDescription = newcmd.BrokenTemplateDescription

// ChooseTemplate will prompt the user to choose amongst the available templates.
func ChooseTemplate(templates []cmdTemplates.Template, opts display.Options) (cmdTemplates.Template, error) {
	return newcmd.ChooseTemplate(templates, opts)
}

