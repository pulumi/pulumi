// Copyright 2025, Pulumi Corporation.

package cli

import (
	"fmt"

	"github.com/pulumi/esc/cmd/esc/cli/client"
)

// Settings commands (env settings get/set) maintain interface compatibility as
// a subset of env commands (env get/set). Flags in settings commands should exist
// in their parent commands. See env_settings_contract_test.go for some basic validation.
//
// To add a new setting:
//
// 1. Add fields in client/apitype.go:
//    - EnvironmentSettings: MyField bool `json:"myField"`
//    - PatchEnvironmentSettingsRequest: MyField *bool `json:"myField,omitempty"`
//
// 2. Create env_setting_my_field.go with:
//    - const settingMyField settingName = "my-field"
//    - MyFieldSetting struct implementing Setting[T] interface
//    (see env_setting_deletion_protected.go for example)
//
// 3. Register in NewEnvSettingsRegistry function:
//    settingMyField: box(&MyFieldSetting{}),

type settingName string

type EnvSettingsRegistry struct {
	Settings map[settingName]UntypedSetting
}

func NewEnvSettingsRegistry() *EnvSettingsRegistry {
	return &EnvSettingsRegistry{
		Settings: map[settingName]UntypedSetting{
			settingDeletionProtected: box(&DeletionProtectedSetting{}),
		},
	}
}

func (r *EnvSettingsRegistry) GetSetting(name string) (UntypedSetting, bool) {
	setting, ok := r.Settings[settingName(name)]
	return setting, ok
}

func (r *EnvSettingsRegistry) GetSettingsHelpText() string {
	result := ""
	for name, setting := range r.Settings {
		if result != "" {
			result += "\n"
		}
		result += fmt.Sprintf("  %s  %s", name, setting.HelpText())
	}
	return result
}

type Setting[T any] interface {
	KebabName() string
	HelpText() string
	ValidateValue(raw string) (T, error)
	GetValue(settings *client.EnvironmentSettings) T
	SetValue(req *client.PatchEnvironmentSettingsRequest, value T)
}

// UntypedSetting and settingBox wrap a typed setting to allow homogeneous
// storage in the registry map.
type UntypedSetting = Setting[any]

type settingBox[T any] struct {
	impl Setting[T]
}

func box[T any](typed Setting[T]) UntypedSetting {
	return &settingBox[T]{impl: typed}
}

func (b *settingBox[T]) KebabName() string {
	return b.impl.KebabName()
}

func (b *settingBox[T]) HelpText() string {
	return b.impl.HelpText()
}

func (b *settingBox[T]) ValidateValue(raw string) (any, error) {
	return b.impl.ValidateValue(raw)
}

func (b *settingBox[T]) GetValue(settings *client.EnvironmentSettings) any {
	return b.impl.GetValue(settings)
}

func (b *settingBox[T]) SetValue(req *client.PatchEnvironmentSettingsRequest, value any) {
	if v, ok := value.(T); ok {
		b.impl.SetValue(req, v)
	}
}
