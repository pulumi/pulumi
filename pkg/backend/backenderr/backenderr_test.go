// Copyright 2019, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backenderr

import (
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/registry"
	pulumienv "github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/stretchr/testify/assert"
)

func TestNotFoundErrorIs(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ErrNotFound, registry.ErrNotFound)
}

func TestLoginRequiredErrorsShareText(t *testing.T) {
	t.Parallel()

	help := LoginHelp{TokensURL: "https://app.pulumi.com/user/settings/tokens", PulumiCloud: true}
	want := "Pulumi Cloud login required\n" +
		"  Run `pulumi login` to sign in or create a free account.\n" +
		"  It opens your web browser and waits until you've signed in there.\n" +
		"  To log in without a browser (CI, headless), set PULUMI_ACCESS_TOKEN." +
		" Create a token at https://app.pulumi.com/user/settings/tokens."

	assert.Equal(t, want, LoginRequiredError{LoginHelp: help}.Error())
	assert.Equal(t, want, MissingEnvVarForNonInteractiveError{
		Var:       pulumienv.AccessToken.Var(),
		LoginHelp: help,
	}.Error())
	assert.Equal(t, want, LoginRequiredMessage(help))
}

func TestLoginRequiredErrorSelfHosted(t *testing.T) {
	t.Parallel()

	// The CLI found the console, so `pulumi login` opens a browser, but there's no free account to create.
	assert.Equal(t, "Pulumi Cloud login required\n"+
		"  Run `pulumi login` to sign in.\n"+
		"  It opens your web browser and waits until you've signed in there.\n"+
		"  To log in without a browser (CI, headless), set PULUMI_ACCESS_TOKEN."+
		" Create a token at https://app.pulumi.example.com/user/settings/tokens.",
		LoginRequiredError{LoginHelp: LoginHelp{
			TokensURL: "https://app.pulumi.example.com/user/settings/tokens",
		}}.Error())
}

func TestLoginRequiredErrorWithoutConsoleURL(t *testing.T) {
	t.Parallel()

	// Without the console's address, `pulumi login` asks for a token instead of opening a browser.
	want := "Pulumi Cloud login required\n" +
		"  Run `pulumi login` to sign in.\n" +
		"  To log in without a browser (CI, headless), set PULUMI_ACCESS_TOKEN."
	assert.Equal(t, want, LoginRequiredError{}.Error())
	assert.Equal(t, want, MissingEnvVarForNonInteractiveError{Var: pulumienv.AccessToken.Var()}.Error())
}

func TestLoginRequiredErrorKeepsSAMLReauthText(t *testing.T) {
	t.Parallel()

	err := LoginRequiredError{
		ReauthURL: "https://app.pulumi.com/signin/sso/acme/reauth",
		LoginHelp: LoginHelp{TokensURL: "https://app.pulumi.com/user/settings/tokens", PulumiCloud: true},
	}
	assert.Equal(t,
		"SAML SSO authentication is required. Log in at:\n\n    https://app.pulumi.com/signin/sso/acme/reauth\n",
		err.Error())
}

func TestLoginRequiredErrorsStayDistinctTypes(t *testing.T) {
	t.Parallel()

	// Callers branch on the type, not the text, so sharing the text must not make one match the other.
	missingEnvVar := MissingEnvVarForNonInteractiveError{Var: pulumienv.AccessToken.Var()}
	assert.NotErrorIs(t, missingEnvVar, ErrLoginRequired)
	assert.NotErrorIs(t, LoginRequiredError{}, MissingEnvVarForNonInteractiveError{})
	assert.ErrorIs(t, LoginRequiredError{LoginHelp: LoginHelp{TokensURL: "https://example.com"}}, ErrLoginRequired)
	assert.True(t, IsAuthError(missingEnvVar))
	assert.True(t, IsAuthError(LoginRequiredError{}))
}
