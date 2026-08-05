package serve

import (
	"context"
	"fmt"
	"strings"

	"vcode/internal/event"
	"vcode/internal/verify"
)

// shouldUsePipeline keeps short requests on the fast single-turn path. The
// pipeline is intentionally conservative: it is for changes that are likely
// to span files or require a verification/debug pass.
func shouldUsePipeline(input string) bool {
	lower := strings.ToLower(strings.TrimSpace(input))
	if len(lower) >= 180 {
		return true
	}
	for _, marker := range []string{"重构", "迁移", "多文件", "同时", "并且", "refactor", "migrate", "multiple files", "end-to-end", "全流程"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func (s *Server) pipelineActive(id string) bool {
	s.pipelineMu.Lock()
	defer s.pipelineMu.Unlock()
	_, ok := s.pipelines[id]
	return ok
}

func (s *Server) beginPipeline(id string) {
	s.pipelineMu.Lock()
	s.pipelines[id] = struct{}{}
	s.pipelineMu.Unlock()
}

func (s *Server) endPipeline(id string) {
	s.pipelineMu.Lock()
	delete(s.pipelines, id)
	s.pipelineMu.Unlock()
}

func (s *Server) runPipeline(id, goal string) {
	ctx := context.Background()
	ctrl := s.ctl()
	runStage := func(role, prompt string, planMode bool) error {
		s.bc.Emit(event.Event{Kind: event.Phase, Text: role})
		ctrl.SetPlanMode(planMode)
		return ctrl.Run(ctx, prompt)
	}

	// Explorer and Planner are read-only turns. Their findings remain in the
	// same session, so Builder receives the real project context without a
	// second hidden storage channel.
	if err := runStage("Explorer", "请先只读研究当前项目，确认相关模块、文件、依赖和现有测试。不要修改文件。用中文给出关键发现。\n\n目标："+goal, true); err != nil {
		s.finishPipeline(id, TaskFailed, "explorer_failed", err.Error())
		return
	}
	if err := runStage("Planner", "请基于刚才的只读研究，用中文制定 2-6 步执行计划。每步写清文件范围、具体动作和验证方式。不要修改文件。\n\n目标："+goal, true); err != nil {
		s.finishPipeline(id, TaskFailed, "planner_failed", err.Error())
		return
	}
	if err := runStage("Builder", "现在执行上面的计划，真实修改项目文件。保持范围最小，完成后说明修改文件和验证方式。\n\n原始目标："+goal, false); err != nil {
		s.finishPipeline(id, TaskFailed, "builder_failed", err.Error())
		return
	}

	result := verify.Run(ctx, ctrl.WorkspaceRoot())
	_ = s.tasks.setVerification(id, string(result.Status))
	for attempt := 1; result.Status != verify.Verified && attempt <= 2; attempt++ {
		_ = s.tasks.setAgent(id, "Debugger", TaskRecovering)
		if err := runStage("Debugger", fmt.Sprintf("验证第 %d 次未通过。请根据失败证据定位根因并修复，只修改必要文件。完成后说明修复内容。\n失败证据：%s", attempt, result.Error()), false); err != nil {
			s.finishPipeline(id, TaskPartial, "debugger_failed", err.Error())
			return
		}
		result = verify.Run(ctx, ctrl.WorkspaceRoot())
		_ = s.tasks.setVerification(id, string(result.Status))
	}
	if false && result.Status != verify.Verified {
		_ = runStage("Debugger", "验证没有完全通过。请根据验证失败证据定位根因并修复，只修改必要文件。完成后说明修复内容。\n\n失败证据："+result.Error(), false)
		result = verify.Run(ctx, ctrl.WorkspaceRoot())
		_ = s.tasks.setVerification(id, string(result.Status))
	}
	if result.Status != verify.Verified {
		s.finishPipeline(id, TaskPartial, "verification_failed", result.Error())
		return
	}
	if err := runStage("Reviewer", "请只读检查当前 Diff、越界修改和回归风险。确认验证证据充分后，用中文给出简短结论，不要修改文件。", true); err != nil {
		s.finishPipeline(id, TaskPartial, "review_failed", err.Error())
		return
	}
	// Run one final check after the review turn so the completion status is
	// backed by fresh evidence rather than by the model's final wording.
	result = verify.Run(ctx, ctrl.WorkspaceRoot())
	_ = s.tasks.setVerification(id, string(result.Status))
	if result.Status == verify.Verified {
		s.finishPipeline(id, TaskCompleted, "", "")
	} else {
		s.finishPipeline(id, TaskPartial, "verification_failed", result.Error())
	}
}

func (s *Server) finishPipeline(id string, status TaskStatus, class, message string) {
	if status == TaskCompleted {
		if decision, err := s.tasks.complete(id); err != nil {
			_ = s.tasks.update(id, TaskPartial, "completion_gate", err.Error())
		} else if !decision.Allowed {
			_ = s.tasks.audit(id, "completion_rejected", map[string]string{"reasons": strings.Join(decision.Reasons, "; ")})
		}
	} else {
		_ = s.tasks.update(id, status, class, message)
	}
	s.endPipeline(id)
	s.bc.SetActiveTask("")
	s.finishTask(id)
}
