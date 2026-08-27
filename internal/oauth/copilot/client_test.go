package copilot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitiatorTransportSetsHeader(t *testing.T) {
	tests := []struct {
		name string
		// body builds the request body; nil means a bodyless request.
		body func() (int, string)
		want string
	}{
		{
			name: "nil body defaults to user",
			body: func() (int, string) { return 0, "" }, // req.Body == nil
			want: "user",
		},
		{
			name: "NoBody defaults to user",
			body: func() (int, string) { return -1, "" }, // req.Body == http.NoBody
			want: "user",
		},
		{
			name: "user-only history stays user",
			body: func() (int, string) { return 1, `{"messages":[{"role":"user"}]}` },
			want: "user",
		},
		{
			name: "assistant history becomes agent",
			body: func() (int, string) { return 1, `{"messages":[{"role":"assistant"}]}` },
			want: "agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("X-Initiator")
			}))
			defer srv.Close()

			kind, payload := tt.body()
			var req *http.Request
			var err error
			switch kind {
			case 0:
				req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
			case -1:
				req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, http.NoBody)
			default:
				req, err = http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, strings.NewReader(payload))
			}
			require.NoError(t, err)

			client := &http.Client{Transport: &initiatorTransport{}}
			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			require.Equal(t, tt.want, got)
		})
	}
}

// TestClientOverridesAuthorizationWithGithubToken locks in the fix for
// the "IDE token expired" 401 loop: the ghToken TokenAccessor is
// consulted on every outbound request and its return value clobbers
// whatever Authorization header the OpenAI SDK put there. Without this
// override the transport would forward the short-lived IDE-token
// Bearer, which the Copilot server rejects when its exp field has
// passed.
func TestClientOverridesAuthorizationWithGithubToken(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	client := NewClient(false, false, func() string { return "ghu_TESTTOKEN" })

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	// Simulate what openai-go / openaicompat.WithAPIKey would have set:
	// a Bearer of the (expired) IDE token.
	req.Header.Set("Authorization", "Bearer tid=18;exp=0;stale")

	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "Bearer ghu_TESTTOKEN", got,
		"transport must overwrite the SDK-set Authorization with the GitHub OAuth token")
}

// TestClientOverridesAuthorizationOnPostBody exercises the code path
// used for actual chat/completions requests — same override logic but
// after the X-Initiator body inspection reads and rebuilds the body.
func TestClientOverridesAuthorizationOnPostBody(t *testing.T) {
	var gotAuth, gotInitiator, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotInitiator = r.Header.Get("X-Initiator")
		b := make([]byte, 512)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
	}))
	defer srv.Close()

	client := NewClient(false, false, func() string { return "ghu_LIVE" })

	body := `{"messages":[{"role":"assistant"}]}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer tid=18;exp=0;stale")

	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "Bearer ghu_LIVE", gotAuth,
		"POST path must also overwrite Authorization")
	require.Equal(t, "agent", gotInitiator,
		"X-Initiator must still be set from body inspection")
	require.Equal(t, body, gotBody,
		"body must be rebuilt after inspection so downstream sees the original bytes")
}

// TestClientLeavesAuthorizationAloneWhenAccessorNil covers the fallback
// path used before any Copilot provider is configured: passing a nil
// TokenAccessor preserves whatever Authorization header the caller set.
// Without this guard the transport would strip auth from callers that
// legitimately want to use another credential shape.
func TestClientLeavesAuthorizationAloneWhenAccessorNil(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	client := NewClient(false, false, nil)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer preserved")

	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "Bearer preserved", got,
		"nil TokenAccessor must leave the caller's Authorization alone")
}

// TestClientLeavesAuthorizationAloneWhenAccessorReturnsEmpty covers the
// transitional state where Copilot is configured but no token is
// available yet (e.g. between login start and callback completion).
// Falling through means the OpenAI SDK's own key stays in place so
// pre-login model list requests still work.
func TestClientLeavesAuthorizationAloneWhenAccessorReturnsEmpty(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	client := NewClient(false, false, func() string { return "" })

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer preserved")

	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "Bearer preserved", got,
		"empty TokenAccessor return must leave the caller's Authorization alone")
}

// TestClientCallsAccessorFreshOnEveryRequest locks in the no-cache
// invariant: the accessor must be called for every outbound request so
// a live re-login (or a plugin rewriting the token) is picked up
// immediately without rebuilding the client. Caching would recreate
// the exact staleness bug this fix eliminates.
func TestClientCallsAccessorFreshOnEveryRequest(t *testing.T) {
	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = append(received, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	var counter atomic.Int32
	client := NewClient(false, false, func() string {
		n := counter.Add(1)
		return "ghu_TOKEN_" + string(rune('0'+n))
	})

	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	require.Equal(t, int32(3), counter.Load(),
		"accessor must be invoked once per request; caching would re-introduce staleness")
	require.Equal(t,
		[]string{"Bearer ghu_TOKEN_1", "Bearer ghu_TOKEN_2", "Bearer ghu_TOKEN_3"},
		received,
		"each request must carry the accessor's fresh return value")
}

