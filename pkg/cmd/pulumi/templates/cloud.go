package templates

import templates "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/templates"

type TemplateMatchable = templates.TemplateMatchable

func NewTemplateMatcher(templateName string) (func(TemplateMatchable) bool, error) {
	return templates.NewTemplateMatcher(templateName)
}

