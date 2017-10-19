package apitype

import "fmt"

/**
 * Go type declarations for REST objects returned from the Pulumi Console API.
 */

// User represents a Pulumi user.
type User struct {
	ID            string        `json:"id"`
	GitHubLogin   string        `json:"githubLogin"`
	Name          string        `json:"name"`
	AvatarURL     string        `json:"avatarUrl"`
	Organizations []interface{} `json:"organizations"`
}

// ErrorResponse is returned from the API when an actual response body is not appropriate. i.e.
// in all error situations.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the Error interface.
func (err ErrorResponse) Error() string {
	return fmt.Sprintf("[%d] %s", err.Code, err.Message)
}
