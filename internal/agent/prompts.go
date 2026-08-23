package agent

import (
	"context"
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

//go:embed templates/plan.md.tpl
var planPromptSuffix []byte

//go:embed templates/plan-read-only.md.tpl
var planReadOnlyPromptSuffix []byte

// BuiltinPromptSuffix returns the embedded prompt-suffix for built-in agents
// that don't set one via config. Empty string means no suffix.
//
// This lets us keep the plan/plan-read-only reminders in .tpl files under
// version control instead of stuffing multi-KB strings into config.go.
func BuiltinPromptSuffix(agentID string) string {
	switch agentID {
	case config.AgentPlan:
		return string(planPromptSuffix)
	case config.AgentPlanReadOnly:
		return string(planReadOnlyPromptSuffix)
	}
	return ""
}

func coderPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("coder", string(coderPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

// agentPrompt returns the system prompt for the given primary agent config.
// All primary agents currently share the coder base template; per-agent
// behaviour comes from config.Agent.PromptSuffix (or the built-in
// BuiltinPromptSuffix fallback) appended at Build-time.
func agentPrompt(agentCfg config.Agent, opts ...prompt.Option) (*prompt.Prompt, error) {
	suffix := agentCfg.PromptSuffix
	if suffix == "" {
		suffix = BuiltinPromptSuffix(agentCfg.ID)
	}
	tmpl := string(coderPromptTmpl)
	if suffix != "" {
		tmpl = tmpl + "\n\n" + suffix
	}
	systemPrompt, err := prompt.NewPrompt(agentCfg.ID, tmpl, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(opts ...prompt.Option) (*prompt.Prompt, error) {
	systemPrompt, err := prompt.NewPrompt("task", string(taskPromptTmpl), opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	systemPrompt, err := prompt.NewPrompt("initialize", string(initializePromptTmpl))
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}
