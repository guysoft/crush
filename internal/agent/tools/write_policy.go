package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
)

// EnforceWriteAllowlist rejects file-writing tool calls whose target path is
// outside the acting agent's WritePathAllowlist. Called at the top of every
// write-capable tool (write, edit, multiedit).
//
// The allowlist is read from ctx via GetWriteAllowlistFromContext:
//
//   - ok=false                         → no policy set, allow (back-compat)
//   - ok=true, patterns == nil / empty → deny all writes
//   - ok=true, non-empty patterns      → allow iff filePath matches one
//
// Glob semantics: `**` matches any run of path segments (including
// slashes); `*` matches one segment. Paths and patterns are resolved
// against workingDir if not absolute.
//
// Returns a fantasy.ToolResponse ready to short-circuit the tool call
// plus a boolean indicating whether the caller should abort. Happy path
// returns zero values.
func EnforceWriteAllowlist(ctx context.Context, workingDir, filePath string) (fantasy.ToolResponse, bool) {
	patterns, ok := GetWriteAllowlistFromContext(ctx)
	if !ok {
		return fantasy.ToolResponse{}, false
	}
	if allowed, err := writeAllowlistMatches(workingDir, patterns, filePath); err != nil {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("denied by agent write policy: %v", err),
		), true
	} else if !allowed {
		agentID := GetAgentIDFromContext(ctx)
		who := "this agent"
		if agentID != "" {
			who = fmt.Sprintf("the %s agent", agentID)
		}
		msg := fmt.Sprintf(
			"denied by agent write policy: %q is not writable by %s. ",
			filePath, who,
		)
		if len(patterns) == 0 {
			msg += "This agent may not write to any files. Switch to the coder agent (Tab / /coder) to edit files."
		} else {
			msg += fmt.Sprintf(
				"This agent may only write to: %s. Switch to the coder agent (Tab / /coder) to edit other files.",
				strings.Join(patterns, ", "),
			)
		}
		return fantasy.NewTextErrorResponse(msg), true
	}
	return fantasy.ToolResponse{}, false
}

// writeAllowlistMatches reports whether filePath matches any pattern in
// patterns, evaluated relative to workingDir. See EnforceWriteAllowlist
// for semantics.
func writeAllowlistMatches(workingDir string, patterns []string, filePath string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}
	target := filePath
	if !filepath.IsAbs(target) {
		target = filepath.Join(workingDir, target)
	}
	target = filepath.Clean(target)

	for _, p := range patterns {
		full := p
		if !filepath.IsAbs(full) {
			full = filepath.Join(workingDir, p)
		}
		full = filepath.Clean(full)
		match, err := doubleStarMatch(full, target)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// doubleStarMatch is a minimal glob matcher supporting `**` (any number
// of path segments) and `*` (any run of chars within one segment). Good
// enough for tool-level path allowlists; not a full gitignore engine.
func doubleStarMatch(pattern, path string) (bool, error) {
	pat := filepath.ToSlash(pattern)
	p := filepath.ToSlash(path)
	if !strings.Contains(pat, "**") {
		return filepath.Match(pat, p)
	}
	i := strings.Index(pat, "**")
	prefix := pat[:i]
	suffix := pat[i+2:]
	if !strings.HasPrefix(p, prefix) {
		return false, nil
	}
	remaining := p[len(prefix):]
	if strings.ContainsAny(suffix, "*?[") {
		for i := 0; i <= len(remaining); i++ {
			ok, err := filepath.Match(suffix, remaining[i:])
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	return strings.HasSuffix(remaining, suffix), nil
}
