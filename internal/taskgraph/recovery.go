package taskgraph

import (
	"fmt"
	"strings"
)

type FailureClass string

const (
	FailureTransient  FailureClass = "transient"
	FailureTimeout    FailureClass = "timeout"
	FailureAuth       FailureClass = "auth"
	FailurePermission FailureClass = "permission"
	FailureCompile    FailureClass = "compile"
	FailureTest       FailureClass = "test"
	FailureConflict   FailureClass = "conflict"
	FailureCancelled  FailureClass = "cancelled"
	FailureBudget     FailureClass = "budget"
	FailureUnknown    FailureClass = "unknown"
)

type FailureContext struct {
	Task    Task
	Node    Node
	Message string
	Class   FailureClass
}

// ClassifyAgentFailure turns raw tool/model errors into policy inputs. The
// classification is intentionally conservative: unknown failures are never
// silently treated as successful work.
func ClassifyAgentFailure(message string) FailureClass {
	s := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(s, "cancel") || strings.Contains(s, "interrupt"):
		return FailureCancelled
	case strings.Contains(s, "budget") || strings.Contains(s, "max steps") || strings.Contains(s, "token limit"):
		return FailureBudget
	case strings.Contains(s, "401") || strings.Contains(s, "403") || strings.Contains(s, "api key") || strings.Contains(s, "unauthorized"):
		return FailureAuth
	case strings.Contains(s, "permission") || strings.Contains(s, "access denied") || strings.Contains(s, "sandbox"):
		return FailurePermission
	case strings.Contains(s, "conflict") || strings.Contains(s, "cherry-pick") || strings.Contains(s, "merge"):
		return FailureConflict
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline exceeded"):
		return FailureTimeout
	case strings.Contains(s, "compile") || strings.Contains(s, "build failed") || strings.Contains(s, "undefined:"):
		return FailureCompile
	case strings.Contains(s, "test failed") || strings.Contains(s, "tests failed") || strings.Contains(s, "failing test") || strings.Contains(s, "verification failed") || strings.Contains(s, "go test"):
		return FailureTest
	case strings.Contains(s, "connection") || strings.Contains(s, "temporary") || strings.Contains(s, "eof") || strings.Contains(s, "rate limit"):
		return FailureTransient
	default:
		return FailureUnknown
	}
}

// DefaultRecoveryDecision gives the Coordinator a safe baseline. Permanent
// failures pause for operator/model intervention; code failures get a
// read-only diagnostic node; transport failures may retry within the node's
// attempt budget.
func DefaultRecoveryDecision(f FailureContext) CoordinationDecision {
	class := f.Class
	if class == "" {
		class = ClassifyAgentFailure(f.Message)
	}
	if class == FailureCancelled || class == FailureBudget || class == FailureAuth || class == FailurePermission {
		return CoordinationDecision{Reason: fmt.Sprintf("%s 不能自动重试：%s", class, f.Message), Actions: []CoordinationAction{{Kind: ActionWait, Message: f.Message, RequestedBy: "coordinator"}}}
	}
	if (class == FailureTransient || class == FailureTimeout || class == FailureUnknown) && f.Node.Attempt < maxAttempts(f.Node) {
		return CoordinationDecision{Reason: fmt.Sprintf("%s 可重试：%s", class, f.Message), Actions: []CoordinationAction{{Kind: ActionRetryNode, NodeID: f.Node.ID, Message: "within attempt budget", RequestedBy: "coordinator"}}}
	}
	if class == FailureCompile || class == FailureTest || class == FailureConflict {
		id := fmt.Sprintf("diagnose-%s-%d", f.Node.ID, f.Node.Attempt+1)
		return CoordinationDecision{Reason: fmt.Sprintf("将失败转交只读诊断 Agent：%s", f.Message), Actions: []CoordinationAction{{Kind: ActionAddNode, Node: &Node{
			ID: id, Title: "诊断失败并提出修复建议", Role: Review, DependsOn: append([]string(nil), f.Node.DependsOn...), Prompt: "分析失败证据，定位根因并提出可验证修复建议，不得写入文件。", Metadata: map[string]string{"tools": "read,search"}, MaxAttempts: 1,
		}, RequestedBy: "coordinator"}}}
	}
	return CoordinationDecision{Reason: "失败需要人工判断：" + f.Message, Actions: []CoordinationAction{{Kind: ActionWait, Message: f.Message, RequestedBy: "coordinator"}}}
}

func maxAttempts(n Node) int {
	if n.MaxAttempts <= 0 {
		return 2
	}
	return n.MaxAttempts
}

func WithinBudget(b Budget, prompt, output int) bool {
	if b.PromptTokens > 0 && prompt > b.PromptTokens {
		return false
	}
	if b.OutputTokens > 0 && output > b.OutputTokens {
		return false
	}
	return true
}
