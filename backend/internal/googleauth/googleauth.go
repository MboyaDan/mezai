// Package googleauth talks to Google for contributor sign-in (Stage 29,
// D-171): the authorization URL, the code exchange and the ID token check.
// It knows nothing about our accounts; auth.Service decides what a verified
// Google identity may do.
//
// The flow is OpenID Connect authorization code with PKCE (S256) and a
// nonce. The ID token is verified with go-oidc against the issuer's
// published keys: signature, issuer, audience (our client ID), expiry and
// the nonce we sent.
package googleauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// DefaultIssuer is Google's OpenID Connect issuer.
const DefaultIssuer = "https://accounts.google.com"

// ErrUnverifiedEmail means Google has not verified the account's email.
var ErrUnverifiedEmail = errors.New("googleauth: email not verified by Google")

// Config holds the OAuth client from the Google Cloud console.
type Config struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is registered with Google: APP_ORIGIN + /api/v1/auth/google/callback.
	RedirectURL string
	// Issuer is DefaultIssuer except in tests, which run their own.
	Issuer string
}

// Claims is what we take from a verified ID token.
type Claims struct {
	Subject string // Google's stable account ID
	Email   string
	Name    string
}

// Provider is safe for concurrent use. Discovery runs on first use, not at
// start-up, so the API starts even when Google cannot be reached; a failed
// discovery is retried on the next sign-in.
type Provider struct {
	cfg    Config
	client *http.Client

	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// New returns a provider, or nil when the client ID or secret is empty
// (Google sign-in off).
func New(cfg Config) *Provider {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	if cfg.Issuer == "" {
		cfg.Issuer = DefaultIssuer
	}
	return &Provider{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (p *Provider) ctx(ctx context.Context) context.Context {
	return oidc.ClientContext(ctx, p.client)
}

func (p *Provider) discover(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.oauth != nil {
		return p.oauth, p.verifier, nil
	}
	op, err := oidc.NewProvider(p.ctx(ctx), p.cfg.Issuer)
	if err != nil {
		return nil, nil, fmt.Errorf("googleauth: discovery: %w", err)
	}
	p.oauth = &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		RedirectURL:  p.cfg.RedirectURL,
		Endpoint:     op.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	p.verifier = op.Verifier(&oidc.Config{ClientID: p.cfg.ClientID})
	return p.oauth, p.verifier, nil
}

// AuthURL is where the browser goes to choose a Google account.
func (p *Provider) AuthURL(ctx context.Context, state, nonce, verifier string) (string, error) {
	o, _, err := p.discover(ctx)
	if err != nil {
		return "", err
	}
	return o.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "select_account")), nil
}

// Exchange trades the code for tokens and returns the verified claims. The
// access token is not kept: we only need to know who the person is.
func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (Claims, error) {
	o, v, err := p.discover(ctx)
	if err != nil {
		return Claims{}, err
	}
	ctx = p.ctx(ctx)
	tok, err := o.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Claims{}, fmt.Errorf("googleauth: exchange: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return Claims{}, errors.New("googleauth: no id_token in the token response")
	}
	idt, err := v.Verify(ctx, raw)
	if err != nil {
		return Claims{}, fmt.Errorf("googleauth: id_token: %w", err)
	}
	if idt.Nonce != nonce {
		return Claims{}, errors.New("googleauth: nonce does not match")
	}
	var c struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idt.Claims(&c); err != nil {
		return Claims{}, fmt.Errorf("googleauth: claims: %w", err)
	}
	if c.Email == "" || !c.EmailVerified {
		return Claims{}, ErrUnverifiedEmail
	}
	return Claims{Subject: idt.Subject, Email: c.Email, Name: c.Name}, nil
}

// NewVerifier returns a random PKCE code verifier.
func NewVerifier() string { return oauth2.GenerateVerifier() }
