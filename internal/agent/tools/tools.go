package tools

import (
	"bytes"
	"context"
	"html/template"
	"os/exec"
	"testing"

	"charm.land/fantasy"
)

type (
	sessionIDContextKey     string
	messageIDContextKey     string
	supportsImagesKey       string
	modelNameKey            string
	agentIDContextKey       string
	writeAllowlistKey       string
)

const (
	// SessionIDContextKey is the key for the session ID in the context.
	SessionIDContextKey sessionIDContextKey = "session_id"
	// MessageIDContextKey is the key for the message ID in the context.
	MessageIDContextKey messageIDContextKey = "message_id"
	// SupportsImagesContextKey is the key for the model's image support capability.
	SupportsImagesContextKey supportsImagesKey = "supports_images"
	// ModelNameContextKey is the key for the model name in the context.
	ModelNameContextKey modelNameKey = "model_name"
	// AgentIDContextKey is the key for the acting agent ID in the context.
	// Set by the coordinator so tools can enforce per-agent policy.
	AgentIDContextKey agentIDContextKey = "agent_id"
	// WriteAllowlistContextKey carries the acting agent's WritePathAllowlist
	// (see config.Agent). Nil = no restriction; empty non-nil = deny all
	// writes; non-empty = allow only paths matching one of the patterns.
	// Consumed by EnforceWriteAllowlist inside the write-capable tools.
	WriteAllowlistContextKey writeAllowlistKey = "write_allowlist"
)

// getContextValue is a generic helper that retrieves a typed value from context.
// If the value is not found or has the wrong type, it returns the default value.
func getContextValue[T any](ctx context.Context, key any, defaultValue T) T {
	value := ctx.Value(key)
	if value == nil {
		return defaultValue
	}
	if typedValue, ok := value.(T); ok {
		return typedValue
	}
	return defaultValue
}

// GetSessionFromContext retrieves the session ID from the context.
func GetSessionFromContext(ctx context.Context) string {
	return getContextValue(ctx, SessionIDContextKey, "")
}

// GetMessageFromContext retrieves the message ID from the context.
func GetMessageFromContext(ctx context.Context) string {
	return getContextValue(ctx, MessageIDContextKey, "")
}

// GetSupportsImagesFromContext retrieves whether the model supports images from the context.
func GetSupportsImagesFromContext(ctx context.Context) bool {
	return getContextValue(ctx, SupportsImagesContextKey, false)
}

// GetModelNameFromContext retrieves the model name from the context.
func GetModelNameFromContext(ctx context.Context) string {
	return getContextValue(ctx, ModelNameContextKey, "")
}

// GetAgentIDFromContext retrieves the acting primary-agent ID from ctx.
// Empty means the coordinator did not stamp one — treat as "no
// agent-level policy applies".
func GetAgentIDFromContext(ctx context.Context) string {
	return getContextValue(ctx, AgentIDContextKey, "")
}

// GetWriteAllowlistFromContext returns the write-path allowlist set by the
// coordinator, if any. `ok=false` means "no policy set for this call" and
// the caller should skip enforcement (backwards compat with pre-agent-switch
// run paths). `ok=true` with an empty slice means "deny all writes"; with
// a non-empty slice means "allow only these globs".
func GetWriteAllowlistFromContext(ctx context.Context) (patterns []string, ok bool) {
	v := ctx.Value(WriteAllowlistContextKey)
	if v == nil {
		return nil, false
	}
	patterns, ok = v.([]string)
	return patterns, ok
}

// NewPermissionDeniedResponse returns a tool response indicating the user
// denied permission, with StopTurn set so the agent loop does not retry.
func NewPermissionDeniedResponse() fantasy.ToolResponse {
	resp := fantasy.NewTextErrorResponse("User denied permission")
	resp.StopTurn = true
	return resp
}

// ghAvailable indicates whether the `gh` CLI is available on PATH.
var ghAvailable = func() bool {
	if testing.Testing() {
		return false
	}
	_, err := exec.LookPath("gh")
	return err == nil
}()

// toolDescriptionData is the common data structure for tool description templates.
type toolDescriptionData struct {
	GhAvailable bool
}

// renderToolDescription renders a tool description template with the given data.
func renderToolDescription(tmpl *template.Template) string {
	data := toolDescriptionData{
		GhAvailable: ghAvailable,
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		panic("failed to execute tool description template: " + err.Error())
	}
	return out.String()
}

// renderTemplate renders a Go template with the given data.
func renderTemplate(tmpl *template.Template, data any) string {
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		panic("failed to execute tool description template: " + err.Error())
	}
	return out.String()
}
