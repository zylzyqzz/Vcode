package control

import (
	"strings"

	"vcode/internal/agent"
	"vcode/internal/capability"
)

func (c *Controller) withCapabilityRoute(composed, routeInput string) string {
	if c == nil {
		return composed
	}
	routeInput = strings.TrimSpace(agent.StripTransientUserBlocks(routeInput))
	if routeInput == "" {
		routeInput = strings.TrimSpace(agent.StripTransientUserBlocks(composed))
	}
	if routeInput == "" {
		return composed
	}
	tools := c.ToolContractEntries()
	entries := capability.ToolEntries(tools)
	entries = append(entries, capability.SkillEntries(c.Skills(), tools)...)
	decision := capability.Route(routeInput, entries)
	block := capability.RenderTransientBlock(decision)
	if block == "" {
		return composed
	}
	return block + "\n\n" + composed
}
