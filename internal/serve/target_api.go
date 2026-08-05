package serve

import (
	"net/http"
	"strings"

	"vcode/internal/runtime"
)

// cloudTarget is deliberately derived from the active controller on every
// request. That keeps the target picker truthful when the selected model or
// workspace changes without introducing a second task runtime.
func (s *Server) cloudTarget() runtime.RuntimeTarget {
	ctl := s.ctl()
	status := runtime.TargetOnline
	if ctl.Running() {
		status = runtime.TargetBusy
	}
	return runtime.RuntimeTarget{
		ID:        "cloud",
		Kind:      runtime.TargetCloud,
		Name:      "Vcode Cloud",
		Status:    status,
		Model:     ctl.Label(),
		Workspace: ctl.SessionDir(),
		Features:  []string{"tasks", "sessions", "diff", "verification", "mcp", "skills"},
	}
}

func (s *Server) apiTargets(w http.ResponseWriter, _ *http.Request) {
	targets := []runtime.RuntimeTarget{s.cloudTarget()}
	if s.relay != nil {
		targets = append(targets, s.relay.snapshot()...)
	}
	writeJSON(w, map[string]any{"targets": targets})
}

func (s *Server) apiTarget(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "cloud" {
		writeJSON(w, s.cloudTarget())
		return
	}
	for _, target := range s.relay.snapshot() {
		if target.ID == id {
			writeJSON(w, target)
			return
		}
	}
	http.NotFound(w, r)
}
