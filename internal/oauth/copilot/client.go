// Package copilot provides GitHub Copilot integration.
package copilot

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/charmbracelet/crush/internal/log"
)

var assistantRolePattern = regexp.MustCompile(`"role"\s*:\s*"assistant"`)

// TokenAccessor returns the GitHub OAuth token (ghu_...) to send as the
// Bearer credential on every Copilot API request. Called once per request
// so a live re-login is picked up without rebuilding the client.
//
// Return an empty string to skip the override and let whatever
// Authorization header the caller (typically the OpenAI SDK) set stand.
type TokenAccessor func() string

// NewClient creates a new HTTP client with a custom transport that:
//
//  1. Sets the X-Initiator header from the message-history shape.
//  2. Overrides the Authorization header with a Bearer of the GitHub
//     OAuth token supplied by ghToken, bypassing the short-lived
//     "IDE token" (tid=...;exp=...) exchange that upstream crush uses.
//     The Copilot chat endpoint accepts either credential (verified) and
//     the GitHub OAuth token does not expire on the request path, so
//     using it directly eliminates the in-flight refresh loop that
//     otherwise wedges after the IDE token's exp field passes. Opencode
//     uses the same approach — see
//     packages/opencode/src/plugin/github-copilot/copilot.ts.
//
// ghToken may be nil for legacy callers; when nil the transport keeps
// whatever Authorization header the caller set.
func NewClient(isSubAgent, debug bool, ghToken TokenAccessor) *http.Client {
	return &http.Client{
		Transport: &initiatorTransport{
			debug:      debug,
			isSubAgent: isSubAgent,
			ghToken:    ghToken,
		},
	}
}

type initiatorTransport struct {
	debug      bool
	isSubAgent bool
	ghToken    TokenAccessor
}

func (t *initiatorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const (
		xInitiatorHeader = "X-Initiator"
		userInitiator    = "user"
		agentInitiator   = "agent"
	)

	if req == nil {
		return nil, fmt.Errorf("HTTP request is nil")
	}
	if req.Body == nil || req.Body == http.NoBody {
		// No body to inspect; default to user. A nil Body is valid for
		// bodyless requests (e.g. GET), and is distinct from http.NoBody,
		// so both must be handled before reading below.
		req.Header.Set(xInitiatorHeader, userInitiator)
		slog.Debug("Setting X-Initiator header to user (no request body)")
		t.applyGithubBearer(req)
		return t.roundTrip(req)
	}

	// Clone request to avoid modifying the original.
	req = req.Clone(req.Context())

	// Read the original body into bytes so we can examine it.
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	defer req.Body.Close()

	// Restore the original body using the preserved bytes.
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Check for assistant messages using regex to handle whitespace
	// variations in the JSON while avoiding full unmarshalling overhead.
	initiator := userInitiator
	if assistantRolePattern.Match(bodyBytes) || t.isSubAgent {
		slog.Debug("Setting X-Initiator header to agent (found assistant messages in history)")
		initiator = agentInitiator
	} else {
		slog.Debug("Setting X-Initiator header to user (no assistant messages)")
	}
	req.Header.Set(xInitiatorHeader, initiator)

	t.applyGithubBearer(req)
	return t.roundTrip(req)
}

// applyGithubBearer overwrites any Authorization header the upstream
// (e.g. the OpenAI SDK's WithAPIKey) placed on the request with a fresh
// `Bearer <ghToken>`. Uses req.Header.Del to clear case variants like
// "authorization" that request cloning may have preserved separately.
func (t *initiatorTransport) applyGithubBearer(req *http.Request) {
	if t.ghToken == nil {
		return
	}
	tok := t.ghToken()
	if tok == "" {
		return
	}
	// Delete any pre-existing casing variant.
	req.Header.Del("Authorization")
	req.Header.Del("authorization")
	req.Header.Set("Authorization", "Bearer "+tok)
}

func (t *initiatorTransport) roundTrip(req *http.Request) (*http.Response, error) {
	if t.debug {
		return log.NewHTTPClient().Transport.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}
