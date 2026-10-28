package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleAuth is the slice of OpenID Connect the HTTP layer needs; tests fake it.
type GoogleAuth interface {
	// AuthCodeURL is where to send the browser. The PKCE verifier stays server-side.
	AuthCodeURL(state, verifier string) string
	// Exchange trades the callback code for a verified identity.
	Exchange(ctx context.Context, code, verifier string) (GoogleIdentity, error)
}

// Google implements the authorization-code flow with PKCE against Google, and
// verifies the returned ID token's signature, issuer, audience and expiry.
type Google struct {
	oauth oauth2.Config

	once     sync.Once
	verifier *oidc.IDTokenVerifier
	initErr  error
}

func NewGoogle(clientID, clientSecret, redirectURL string) *Google {
	return &Google{oauth: oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     google.Endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}}
}

func (g *Google) AuthCodeURL(state, verifier string) string {
	return g.oauth.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

func (g *Google) Exchange(ctx context.Context, code, verifier string) (GoogleIdentity, error) {
	// Discovery is lazy, so the server starts even if Google is unreachable.
	g.once.Do(func() {
		provider, err := oidc.NewProvider(context.WithoutCancel(ctx), "https://accounts.google.com")
		if err != nil {
			g.initErr = fmt.Errorf("google discovery: %w", err)
			return
		}
		g.verifier = provider.Verifier(&oidc.Config{ClientID: g.oauth.ClientID})
	})
	if g.initErr != nil {
		return GoogleIdentity{}, g.initErr
	}

	tok, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("google code exchange: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok {
		return GoogleIdentity{}, errors.New("google response had no id_token")
	}
	idToken, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("verify google id_token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return GoogleIdentity{}, fmt.Errorf("read google claims: %w", err)
	}
	return GoogleIdentity{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
	}, nil
}
