// Package auth is a test fixture for the Encore scraper.
package auth

import "context"

type (
	// LoginReq is the request body for the login endpoint.
	LoginReq struct{ Email, Password string }
	// LoginResp is the response body for the login endpoint.
	LoginResp struct{ Token string }
)

// Login authenticates a user and returns a session token.
//
//encore:api public method=POST path=/auth/login
func Login(_ context.Context, _ *LoginReq) (*LoginResp, error) {
	return &LoginResp{Token: "fake"}, nil
}
