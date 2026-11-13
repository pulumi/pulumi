// Copyright 2026, Pulumi Corporation.
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

package awssetup

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidc_types "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	awssetup_types "github.com/pulumi/pulumi/pkg/v3/cloudsetup/awssetup/types"
)

type fakeSsoOidcClient struct {
	registerErr  error
	startAuthErr error
	createErr    error
}

func (f *fakeSsoOidcClient) RegisterClient(
	context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options),
) (*ssooidc.RegisterClientOutput, error) {
	if f.registerErr != nil {
		return nil, f.registerErr
	}
	return &ssooidc.RegisterClientOutput{
		ClientId:     aws.String("client123"),
		ClientSecret: aws.String("secret456"),
	}, nil
}

func (f *fakeSsoOidcClient) StartDeviceAuthorization(
	context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options),
) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	return nil, f.startAuthErr
}

func (f *fakeSsoOidcClient) CreateToken(
	context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options),
) (*ssooidc.CreateTokenOutput, error) {
	return nil, f.createErr
}

func newFakeSSOClient(oidc *fakeSsoOidcClient) *ssoClient {
	return &ssoClient{awsCfg: aws.Config{Region: "us-west-2"}, ssoOidcClient: oidc}
}

func TestSSOClientInitiateErrors(t *testing.T) {
	t.Parallel()

	const startURL = "https://d-1234567890.awsapps.com/start"

	t.Run("surfaces the AWS description when the start URL is rejected", func(t *testing.T) {
		t.Parallel()
		client := newFakeSSOClient(&fakeSsoOidcClient{
			startAuthErr: &smithy.OperationError{
				ServiceID:     "SSO OIDC",
				OperationName: "StartDeviceAuthorization",
				Err: &ssooidc_types.InvalidRequestException{
					Error_:            aws.String("invalid_request"),
					Error_description: aws.String("Invalid start URL"),
				},
			},
		})

		result, err := client.Initiate(t.Context(), startURL)

		assert.Nil(t, result)
		assert.EqualError(t, err, "Invalid start URL")
	})

	t.Run("explains a network failure in terms of the SSO region", func(t *testing.T) {
		t.Parallel()
		// Shape of a DNS failure as the SDK reports it: the operation error wraps the
		// transport error, which wraps the *net.DNSError.
		client := newFakeSSOClient(&fakeSsoOidcClient{
			registerErr: &smithy.OperationError{
				ServiceID:     "SSO OIDC",
				OperationName: "RegisterClient",
				Err: &url.Error{
					Op:  "Post",
					URL: "https://oidc.us-west-2.amazonaws.com/client/register",
					Err: &net.DNSError{Name: "oidc.us-west-2.amazonaws.com", IsNotFound: true},
				},
			},
		})

		result, err := client.Initiate(t.Context(), startURL)

		assert.Nil(t, result)
		assert.EqualError(t, err,
			`could not reach the AWS IAM Identity Center (SSO OIDC) endpoint for region "us-west-2"; `+
				"verify the SSO region and try again")
	})

	t.Run("reports a timeout without blaming the SSO region", func(t *testing.T) {
		t.Parallel()
		// A client-level timeout surfaces as a *url.Error wrapping context.DeadlineExceeded,
		// which satisfies net.Error just like a DNS failure does.
		client := newFakeSSOClient(&fakeSsoOidcClient{
			registerErr: &smithy.OperationError{
				ServiceID:     "SSO OIDC",
				OperationName: "RegisterClient",
				Err: &url.Error{
					Op:  "Post",
					URL: "https://oidc.us-west-2.amazonaws.com/client/register",
					Err: context.DeadlineExceeded,
				},
			},
		})

		result, err := client.Initiate(t.Context(), startURL)

		assert.Nil(t, result)
		assert.EqualError(t, err,
			`timed out contacting the AWS IAM Identity Center (SSO OIDC) endpoint for region "us-west-2"; `+
				"please try again")
	})
}

func TestSSOClientExchangeAccessTokenErrors(t *testing.T) {
	t.Parallel()

	t.Run("classifies the device-authorization errors", func(t *testing.T) {
		t.Parallel()
		classified := []struct {
			name     string
			err      error
			sentinel error
		}{
			{"pending", &ssooidc_types.AuthorizationPendingException{}, ErrSSOAuthorizationPending},
			{"slow down", &ssooidc_types.SlowDownException{}, ErrSSOSlowDown},
			{"expired", &ssooidc_types.ExpiredTokenException{}, ErrSSOAuthorizationExpired},
			{"denied", &ssooidc_types.AccessDeniedException{}, ErrSSOAuthorizationDenied},
			{"invalid client", &ssooidc_types.InvalidClientException{}, ErrSSOAuthorizationRejected},
			{"invalid client metadata", &ssooidc_types.InvalidClientMetadataException{}, ErrSSOAuthorizationRejected},
			{"invalid grant", &ssooidc_types.InvalidGrantException{}, ErrSSOAuthorizationRejected},
			{"invalid request", &ssooidc_types.InvalidRequestException{}, ErrSSOAuthorizationRejected},
			{"invalid request region", &ssooidc_types.InvalidRequestRegionException{}, ErrSSOAuthorizationRejected},
			{"invalid scope", &ssooidc_types.InvalidScopeException{}, ErrSSOAuthorizationRejected},
			{"unauthorized client", &ssooidc_types.UnauthorizedClientException{}, ErrSSOAuthorizationRejected},
			{"unsupported grant type", &ssooidc_types.UnsupportedGrantTypeException{}, ErrSSOAuthorizationRejected},
		}
		for _, tc := range classified {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := newFakeSSOClient(&fakeSsoOidcClient{createErr: tc.err})

				_, err := client.ExchangeAccessToken(t.Context(), &awssetup_types.SSOConfig{})

				require.ErrorIs(t, err, tc.sentinel)
				assert.ErrorIs(t, err, tc.err)
			})
		}
	})

	t.Run("does not classify errors that may be transient", func(t *testing.T) {
		t.Parallel()
		for _, awsErr := range []error{
			&ssooidc_types.InternalServerException{},
			errors.New("dial tcp: i/o timeout"),
		} {
			t.Run(awsErr.Error(), func(t *testing.T) {
				t.Parallel()
				client := newFakeSSOClient(&fakeSsoOidcClient{createErr: awsErr})

				_, err := client.ExchangeAccessToken(t.Context(), &awssetup_types.SSOConfig{})

				assert.ErrorIs(t, err, awsErr)
				assert.NotErrorIs(t, err, ErrSSOAuthorizationPending)
				assert.NotErrorIs(t, err, ErrSSOSlowDown)
				assert.NotErrorIs(t, err, ErrSSOAuthorizationExpired)
				assert.NotErrorIs(t, err, ErrSSOAuthorizationDenied)
				assert.NotErrorIs(t, err, ErrSSOAuthorizationRejected)
			})
		}
	})
}
