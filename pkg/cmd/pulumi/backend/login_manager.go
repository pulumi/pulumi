package backend

import backend "github.com/pulumi/pulumi/sdk/v3/pkg/cmd/pulumi/backend"

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager = backend.LoginManager

type MockLoginManager = backend.MockLoginManager

var DefaultLoginManager = backend.DefaultLoginManager

