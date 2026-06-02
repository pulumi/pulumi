// Copyright 2024, Pulumi Corporation.  All rights reserved.

/*
ESC (Environments, Secrets, Config) API

Testing EscAPIService

*/

package esc_sdk

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const PROJECT_NAME = "sdk-go-test"
const ENV_PREFIX = "env-"

func Test_EscClient(t *testing.T) {
	orgName := os.Getenv("PULUMI_ORG")
	if orgName == "" {
		t.Skip("PULUMI_ORG must be set to run this integration test")
	}
	auth, apiClient, err := DefaultLogin()
	require.NoError(t, err)

	removeAllGoTestEnvs(t, apiClient, auth, orgName)

	baseEnvName := "base-" + time.Now().Format("20060102150405")
	err = apiClient.CreateEnvironment(auth, orgName, PROJECT_NAME, baseEnvName)
	require.Nil(t, err)
	t.Cleanup(func() {
		err := apiClient.DeleteEnvironment(auth, orgName, PROJECT_NAME, baseEnvName)
		require.Nil(t, err)
	})

	baseEnv := &EnvironmentDefinition{
		Values: &EnvironmentDefinitionValues{
			AdditionalProperties: map[string]any{
				"base": baseEnvName,
			},
		},
	}

	_, err = apiClient.UpdateEnvironment(auth, orgName, PROJECT_NAME, baseEnvName, baseEnv)
	require.Nil(t, err)

	t.Run("should create, clone, list, update, get, decrypt, open and delete an environment", func(t *testing.T) {
		envName := ENV_PREFIX + time.Now().Format("20060102150405")
		err := apiClient.CreateEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)

		cloneProject := fmt.Sprintf("%s-clone", PROJECT_NAME)
		cloneName := fmt.Sprintf("%s-clone", envName)
		err = apiClient.CloneEnvironment(auth, orgName, PROJECT_NAME, envName, cloneProject, cloneName, &CloneEnvironmentOptions{})
		require.Nil(t, err)

		t.Cleanup(func() {
			err := apiClient.DeleteEnvironment(auth, orgName, PROJECT_NAME, envName)
			require.Nil(t, err)
			err = apiClient.DeleteEnvironment(auth, orgName, cloneProject, cloneName)
			require.Nil(t, err)
		})

		_, _, err = apiClient.GetEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err, "created env should exist")
		_, _, err = apiClient.GetEnvironment(auth, orgName, cloneProject, cloneName)
		require.Nil(t, err, "cloned env should exist")

		_, values, err := apiClient.OpenAndReadEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		var nilValues map[string]any = nil
		require.Equal(t, values, nilValues)

		yaml := "imports:\n  - " + PROJECT_NAME + "/" + baseEnvName + "\n" + `
values:
  foo: bar
  my_secret:
    fn::secret: "shh! don't tell anyone"
  my_array: [1, 2, 3]
  pulumiConfig:
    foo: ${foo}
  environmentVariables:
    FOO: ${foo}
`

		diags, err := apiClient.UpdateEnvironmentYaml(auth, orgName, PROJECT_NAME, envName, yaml)
		require.Nil(t, err)
		require.NotNil(t, diags)
		require.Len(t, diags.Diagnostics, 0)

		env, newYaml, err := apiClient.GetEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, newYaml)

		assertEnvDef(t, env, baseEnvName)

		require.NotNil(t, env.Values.AdditionalProperties["my_secret"].(map[string]any))

		decryptEnv, _, err := apiClient.DecryptEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)

		assertEnvDef(t, decryptEnv, baseEnvName)

		mySecret, ok := decryptEnv.Values.AdditionalProperties["my_secret"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "shh! don't tell anyone", mySecret["fn::secret"])

		_, values, err = apiClient.OpenAndReadEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)

		require.Equal(t, baseEnvName, values["base"])
		require.Equal(t, "bar", values["foo"])
		require.Equal(t, []any{1.0, 2.0, 3.0}, values["my_array"])
		require.Equal(t, "shh! don't tell anyone", values["my_secret"])
		pulumiConfig, ok := values["pulumiConfig"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "bar", pulumiConfig["foo"])
		environmentVariables, ok := values["environmentVariables"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "bar", environmentVariables["FOO"])

		openInfo, err := apiClient.OpenEnvironment(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)

		v, value, err := apiClient.ReadEnvironmentProperty(auth, orgName, PROJECT_NAME, envName, openInfo.Id, "pulumiConfig.foo")
		require.Nil(t, err)
		require.Equal(t, "bar", v.Value)
		require.Equal(t, "bar", value)

		env, _, err = apiClient.GetEnvironmentAtVersion(auth, orgName, PROJECT_NAME, envName, "2")
		require.Nil(t, err)

		env.Values.AdditionalProperties["versioned"] = "true"

		_, err = apiClient.UpdateEnvironment(auth, orgName, PROJECT_NAME, envName, env)
		require.Nil(t, err)

		revisions, err := apiClient.ListEnvironmentRevisions(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, revisions)
		require.Len(t, revisions, 3)

		err = apiClient.CreateEnvironmentRevisionTag(auth, orgName, PROJECT_NAME, envName, "testTag", 2)
		require.Nil(t, err)

		_, values, err = apiClient.OpenAndReadEnvironmentAtVersion(auth, orgName, PROJECT_NAME, envName, "testTag")
		require.Nil(t, err)
		_, ok = values["versioned"]
		require.Equal(t, ok, false)

		tags, err := apiClient.ListEnvironmentRevisionTags(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, tags)
		require.Len(t, tags.Tags, 2)
		require.Equal(t, "latest", tags.Tags[0].Name)
		require.Equal(t, "testTag", tags.Tags[1].Name)

		err = apiClient.UpdateEnvironmentRevisionTag(auth, orgName, PROJECT_NAME, envName, "testTag", 3)
		require.Nil(t, err)

		_, values, err = apiClient.OpenAndReadEnvironmentAtVersion(auth, orgName, PROJECT_NAME, envName, "testTag")
		require.Nil(t, err)
		require.Equal(t, "true", values["versioned"])

		testTag, err := apiClient.GetEnvironmentRevisionTag(auth, orgName, PROJECT_NAME, envName, "testTag")
		require.Nil(t, err)
		require.NotNil(t, testTag)
		require.Equal(t, int32(3), testTag.Revision)

		err = apiClient.DeleteEnvironmentRevisionTag(auth, orgName, PROJECT_NAME, envName, "testTag")
		require.Nil(t, err)

		tags, err = apiClient.ListEnvironmentRevisionTags(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, tags)
		require.Len(t, tags.Tags, 1)

		_, err = apiClient.CreateEnvironmentTag(auth, orgName, PROJECT_NAME, envName, "owner", "esc-sdk-test")
		require.Nil(t, err)

		envTags, err := apiClient.ListEnvironmentTags(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, envTags)
		require.Len(t, envTags.Tags, 1)
		require.Equal(t, "owner", envTags.Tags["owner"].Name)
		require.Equal(t, "esc-sdk-test", *envTags.Tags["owner"].Value)

		_, err = apiClient.UpdateEnvironmentTag(auth, orgName, PROJECT_NAME, envName, "owner", "esc-sdk-test", "new-owner", "esc-sdk-test-updated")
		require.Nil(t, err)

		envTag, err := apiClient.GetEnvironmentTag(auth, orgName, PROJECT_NAME, envName, "new-owner")
		require.Nil(t, err)
		require.NotNil(t, envTag)
		require.Equal(t, "new-owner", envTag.Name)
		require.Equal(t, "esc-sdk-test-updated", *envTag.Value)

		err = apiClient.DeleteEnvironmentTag(auth, orgName, PROJECT_NAME, envName, "new-owner")
		require.Nil(t, err)

		envTags, err = apiClient.ListEnvironmentTags(auth, orgName, PROJECT_NAME, envName)
		require.Nil(t, err)
		require.NotNil(t, envTags)
		require.Len(t, envTags.Tags, 0)
	})

	t.Run("check environment definition valid", func(t *testing.T) {
		env := &EnvironmentDefinition{
			Values: &EnvironmentDefinitionValues{
				AdditionalProperties: map[string]any{
					"foo": "bar",
				},
			},
		}

		diags, err := apiClient.CheckEnvironment(auth, orgName, env)
		require.Nil(t, err)
		require.NotNil(t, diags)
		require.Len(t, diags.Diagnostics, 0)
	})

	t.Run("check environment yaml invalid", func(t *testing.T) {
		yaml := `
values:
  foo: bar
  pulumiConfig:
    foo: ${bad_ref}
`
		diags, err := apiClient.CheckEnvironmentYaml(auth, orgName, yaml)
		require.Error(t, err, "400 Bad Request")
		require.NotNil(t, diags)
		require.Len(t, diags.Diagnostics, 1)
		require.Equal(t, "unknown property \"bad_ref\"", diags.Diagnostics[0].Summary)

	})
}

func assertEnvDef(t *testing.T, env *EnvironmentDefinition, baseEnvName string) {
	require.Len(t, env.Imports, 1)
	require.Equal(t, PROJECT_NAME+"/"+baseEnvName, env.Imports[0])
	require.Equal(t, "bar", env.Values.AdditionalProperties["foo"])
	require.Equal(t, []any{1.0, 2.0, 3.0}, env.Values.AdditionalProperties["my_array"])

	require.Equal(t, "${foo}", env.Values.PulumiConfig["foo"])
	require.NotNil(t, env.Values.EnvironmentVariables)
	envVariables := *env.Values.EnvironmentVariables
	require.Equal(t, "${foo}", envVariables["FOO"])
}

func removeAllGoTestEnvs(t *testing.T, apiClient *EscClient, auth context.Context, orgName string) {
	var continuationToken *string
	for {
		envs, err := apiClient.ListEnvironments(auth, orgName, continuationToken)
		require.Nil(t, err)

		if len(envs.Environments) == 0 {
			break
		}
		for _, env := range envs.Environments {
			if env.Project == PROJECT_NAME && strings.HasPrefix(env.Name, ENV_PREFIX) {
				err := apiClient.DeleteEnvironment(auth, orgName, PROJECT_NAME, env.Name)
				require.Nil(t, err)
			}
		}

		continuationToken = envs.NextToken
		if continuationToken == nil || *continuationToken == "" {
			break
		}
	}
}
