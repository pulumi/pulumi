// Copyright 2016-2020, Pulumi Corporation.
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

package pulumi

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/backend/display"
	"github.com/pulumi/pulumi/pkg/v2/backend/pulumi/client"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v2/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
	"github.com/skratchdot/open-golang/open"
)

// WelcomeUser prints a Welcome to Pulumi message.
func WelcomeUser(opts display.Options) {
	fmt.Printf(`

  %s

  Pulumi helps you create, deploy, and manage infrastructure on any cloud using
  your favorite language. You can get started today with Pulumi at:

      https://www.pulumi.com/docs/get-started/

  %s Resources you create with Pulumi are given unique names (a randomly
  generated suffix) by default. To learn more about auto-naming or customizing resource
  names see https://www.pulumi.com/docs/intro/concepts/programming-model/#autonaming.


`,
		opts.Color.Colorize(colors.SpecHeadline+"Welcome to Pulumi!"+colors.Reset),
		opts.Color.Colorize(colors.SpecSubHeadline+"Tip of the day:"+colors.Reset))
}

// Login logs into the target cloud URL and returns the client for it.
func Login(ctx context.Context, d diag.Sink, cloudURL string, opts display.Options) (backend.Client, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)

	// If we have a saved access token, and it is valid, use it.
	existingAccount, err := workspace.GetAccount(cloudURL)
	if err == nil && existingAccount.AccessToken != "" {
		// If the account was last verified less than an hour ago, assume the token is valid.
		valid, username := true, existingAccount.Username
		if username == "" || existingAccount.LastValidatedAt.Add(1*time.Hour).Before(time.Now()) {
			valid, username, err = IsValidAccessToken(ctx, cloudURL, existingAccount.AccessToken)
			if err != nil {
				return nil, err
			}
			existingAccount.LastValidatedAt = time.Now()
		}

		if valid {
			// Save the token. While it hasn't changed this will update the current cloud we are logged into, as well.
			existingAccount.Username = username
			if err = workspace.StoreAccount(cloudURL, existingAccount, true); err != nil {
				return nil, err
			}

			return NewClient(d, cloudURL)
		}
	}

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(AccessTokenEnvVar)
	accountLink := cloudConsoleURL(cloudURL, "account", "tokens")

	if accessToken != "" {
		// If there's already a token from the environment, use it.
		_, err = fmt.Fprintf(os.Stderr, "Logging in using access token from %s\n", AccessTokenEnvVar)
		contract.IgnoreError(err)
	} else if !cmdutil.Interactive() {
		// If interactive mode isn't enabled, the only way to specify a token is through the environment variable.
		// Fail the attempt to login.
		return nil, errors.Errorf(
			"%s must be set for login during non-interactive CLI sessions", AccessTokenEnvVar)
	} else {
		// If no access token is available from the environment, and we are interactive, prompt and offer to
		// open a browser to make it easy to generate and use a fresh token.
		line1 := fmt.Sprintf("Manage your Pulumi stacks by logging in.")
		line1len := len(line1)
		line1 = colors.Highlight(line1, "Pulumi stacks", colors.Underline+colors.Bold)
		fmt.Printf(opts.Color.Colorize(line1) + "\n")
		maxlen := line1len

		line2 := "Run `pulumi login --help` for alternative login options."
		line2len := len(line2)
		fmt.Printf(opts.Color.Colorize(line2) + "\n")
		if line2len > maxlen {
			maxlen = line2len
		}

		// In the case where we could not construct a link to the pulumi console based on the API server's hostname,
		// don't offer magic log-in or text about where to find your access token.
		if accountLink == "" {
			for {
				if accessToken, err = cmdutil.ReadConsoleNoEcho("Enter your access token"); err != nil {
					return nil, err
				}
				if accessToken != "" {
					break
				}
			}
		} else {
			line3 := fmt.Sprintf("Enter your access token from %s", accountLink)
			line3len := len(line3)
			line3 = colors.Highlight(line3, "access token", colors.BrightCyan+colors.Bold)
			line3 = colors.Highlight(line3, accountLink, colors.BrightBlue+colors.Underline+colors.Bold)
			fmt.Printf(opts.Color.Colorize(line3) + "\n")
			if line3len > maxlen {
				maxlen = line3len
			}

			line4 := "    or hit <ENTER> to log in using your browser"
			var padding string
			if pad := maxlen - len(line4); pad > 0 {
				padding = strings.Repeat(" ", pad)
			}
			line4 = colors.Highlight(line4, "<ENTER>", colors.BrightCyan+colors.Bold)

			if accessToken, err = cmdutil.ReadConsoleNoEcho(opts.Color.Colorize(line4) + padding); err != nil {
				return nil, err
			}

			if accessToken == "" {
				return loginWithBrowser(ctx, d, cloudURL, opts)
			}

			// Welcome the user since this was an interactive login.
			WelcomeUser(opts)
		}
	}

	// Try and use the credentials to see if they are valid.
	valid, username, err := IsValidAccessToken(ctx, cloudURL, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, errors.Errorf("invalid access token")
	}

	// Save them.
	account := workspace.Account{AccessToken: accessToken, Username: username, LastValidatedAt: time.Now()}
	if err = workspace.StoreAccount(cloudURL, account, true); err != nil {
		return nil, err
	}

	return NewClient(d, cloudURL)
}

// loginWithBrowser uses a web-browser to log into the cloud and returns the cloud backend for it.
func loginWithBrowser(ctx context.Context, d diag.Sink, cloudURL string, opts display.Options) (backend.Client, error) {
	// Locally, we generate a nonce and spin up a web server listening on a random port on localhost. We then open a
	// browser to a special endpoint on the Pulumi.com console, passing the generated nonce as well as the port of the
	// webserver we launched. This endpoint does the OAuth flow and when it completes, redirects to localhost passing
	// the nonce and the pulumi access token we created as part of the OAuth flow. If the nonces match, we set the
	// access token that was passed to us and the redirect to a special welcome page on Pulumi.com

	loginURL := cloudConsoleURL(cloudURL, "cli-login")
	finalWelcomeURL := cloudConsoleURL(cloudURL, "welcome", "cli")

	if loginURL == "" || finalWelcomeURL == "" {
		return nil, errors.New("could not determine login url")
	}

	// Listen on localhost, have the kernel pick a random port for us
	c := make(chan string)
	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return nil, errors.Wrap(err, "could not start listener")
	}

	// Extract the port
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, errors.Wrap(err, "could not determine port")
	}

	// Generate a nonce we'll send with the request.
	nonceBytes := make([]byte, 32)
	_, err = cryptorand.Read(nonceBytes)
	contract.AssertNoErrorf(err, "could not get random bytes")
	nonce := hex.EncodeToString(nonceBytes)

	u, err := url.Parse(loginURL)
	contract.AssertNoError(err)

	// Generate a description to associate with the access token we'll generate, for display on the Account Settings
	// page.
	var tokenDescription string
	if host, hostErr := os.Hostname(); hostErr == nil {
		tokenDescription = fmt.Sprintf("Generated by pulumi login on %s at %s", host, time.Now().Format(time.RFC822))
	} else {
		tokenDescription = fmt.Sprintf("Generated by pulumi login at %s", time.Now().Format(time.RFC822))
	}

	// Pass our state around as query parameters on the URL we'll open the user's preferred browser to
	q := u.Query()
	q.Add("cliSessionPort", port)
	q.Add("cliSessionNonce", nonce)
	q.Add("cliSessionDescription", tokenDescription)
	u.RawQuery = q.Encode()

	// Start the webserver to listen to handle the response
	go serveBrowserLoginServer(l, nonce, finalWelcomeURL, c)

	// Launch the web browser and navigate to the login URL.
	if openErr := open.Run(u.String()); openErr != nil {
		fmt.Printf("We couldn't launch your web browser for some reason. Please visit:\n\n%s\n\n"+
			"to finish the login process.", u)
	} else {
		fmt.Println("We've launched your web browser to complete the login process.")
	}

	fmt.Println("\nWaiting for login to complete...")

	accessToken := <-c

	username, err := client.NewClient(cloudURL, accessToken, d).GetPulumiAccountName(ctx)
	if err != nil {
		return nil, err
	}

	// Save the token and return the backend
	account := workspace.Account{AccessToken: accessToken, Username: username, LastValidatedAt: time.Now()}
	if err = workspace.StoreAccount(cloudURL, account, true); err != nil {
		return nil, err
	}

	// Welcome the user since this was an interactive login.
	WelcomeUser(opts)

	return NewClient(d, cloudURL)
}

func serveBrowserLoginServer(l net.Listener, expectedNonce string, destinationURL string, c chan<- string) {
	handler := func(res http.ResponseWriter, req *http.Request) {
		tok := req.URL.Query().Get("accessToken")
		nonce := req.URL.Query().Get("nonce")

		if tok == "" || nonce != expectedNonce {
			res.WriteHeader(400)
			return
		}

		http.Redirect(res, req, destinationURL, http.StatusTemporaryRedirect)
		c <- tok
	}

	mux := &http.ServeMux{}
	mux.HandleFunc("/", handler)
	contract.IgnoreError(http.Serve(l, mux))
}

// IsValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func IsValidAccessToken(ctx context.Context, cloudURL, accessToken string) (bool, string, error) {
	// Make a request to get the authenticated user. If it returns a successful response,
	// we know the access token is legit. We also parse the response as JSON and confirm
	// it has a githubLogin field that is non-empty (like the Pulumi Service would return).
	username, err := client.NewClient(cloudURL, accessToken, cmdutil.Diag()).GetPulumiAccountName(ctx)
	if err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, "", nil
		}
		return false, "", errors.Wrapf(err, "getting user info from %v", cloudURL)
	}

	return true, username, nil
}
