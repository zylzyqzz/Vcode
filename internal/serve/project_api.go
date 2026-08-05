package serve

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func pathID(r *http.Request, name string) (string, error) {
	id := strings.TrimSpace(r.PathValue(name))
	if !projectIDPattern.MatchString(id) {
		return "", errors.New("invalid identifier")
	}
	return id, nil
}

func (s *Server) apiProjects(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.projects.list())
}

func (s *Server) apiProjectCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		RepositoryURL string `json:"repository_url"`
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid project body", http.StatusBadRequest)
		return
	}
	body.ID = strings.TrimSpace(body.ID)
	if !projectIDPattern.MatchString(body.ID) || strings.TrimSpace(body.RepositoryURL) == "" {
		http.Error(w, "id and repository_url are required", http.StatusBadRequest)
		return
	}
	if err := validateRepositoryURL(body.RepositoryURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := ProjectRecord{ID: body.ID, Name: strings.TrimSpace(body.Name), RepositoryURL: strings.TrimSpace(body.RepositoryURL), Root: filepath.Join(s.projects.root, body.ID), DefaultBranch: strings.TrimSpace(body.DefaultBranch)}
	if err := s.projects.upsert(p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

func (s *Server) apiProjectClone(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, ok := s.projects.get(id)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if !projectPathWithin(s.projects.root, p.Root) {
		http.Error(w, "project path rejected", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(p.Root), 0o700); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cmd := exec.CommandContext(r.Context(), "git", "clone", "--", p.RepositoryURL, p.Root)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			http.Error(w, redactGitOutput(string(output)), http.StatusBadGateway)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(p)
}

func (s *Server) apiProjectBranch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p, ok := s.projects.get(id)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	var body struct {
		TaskID     string `json:"task_id"`
		BaseBranch string `json:"base_branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !projectIDPattern.MatchString(body.TaskID) {
		http.Error(w, "task_id is required", http.StatusBadRequest)
		return
	}
	base := strings.TrimSpace(body.BaseBranch)
	if base == "" {
		base = p.DefaultBranch
	}
	branch := "vcode/" + body.TaskID
	if err := safeBranchName(branch); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	workspace := filepath.Join(p.Root, ".vcode", "worktrees", body.TaskID, "build")
	if !projectPathWithin(p.Root, workspace) {
		http.Error(w, "workspace path rejected", http.StatusBadRequest)
		return
	}
	if _, err := os.Stat(filepath.Join(p.Root, ".git")); err != nil {
		http.Error(w, "clone the project before creating a branch", http.StatusConflict)
		return
	}
	if _, err := runGit(r.Context(), p.Root, "fetch", "--all", "--prune"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := os.MkdirAll(filepath.Dir(workspace), 0o700); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := runGit(r.Context(), p.Root, "worktree", "add", "-b", branch, workspace, base); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err := s.switchWorkspace(r.Context(), workspace); err != nil {
		_, _ = runGit(r.Context(), p.Root, "worktree", "remove", "--force", workspace)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"project_id": id, "task_id": body.TaskID, "branch": branch, "workspace": workspace})
}

func (s *Server) taskWorkspace(id string) (*TaskRecord, error) {
	record, err := s.tasks.record(id)
	if err != nil {
		return nil, err
	}
	if !projectPathWithin(s.projects.root, record.Workspace) {
		return nil, errors.New("task workspace is outside project root")
	}
	return record, nil
}

func (s *Server) apiTaskDiff(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.taskWorkspace(id)
	if err != nil {
		http.Error(w, "task not found or workspace rejected", http.StatusNotFound)
		return
	}
	diff, err := runGit(r.Context(), record.Workspace, "diff", "HEAD")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"task_id": id, "workspace": record.Workspace, "diff": diff})
}

func (s *Server) apiTaskCommit(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.taskWorkspace(id)
	if err != nil {
		http.Error(w, "task not found or workspace rejected", http.StatusNotFound)
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Message) == "" {
		http.Error(w, "commit message is required", http.StatusBadRequest)
		return
	}
	if _, err := runGit(r.Context(), record.Workspace, "add", "-A"); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	commit, err := runGit(r.Context(), record.Workspace, "commit", "-m", body.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	commit, _ = runGit(r.Context(), record.Workspace, "rev-parse", "HEAD")
	_ = s.tasks.audit(id, "git_commit", map[string]string{"commit": commit})
	writeJSON(w, map[string]string{"task_id": id, "commit": commit})
}

func (s *Server) apiTaskPush(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.taskWorkspace(id)
	if err != nil {
		http.Error(w, "task not found or workspace rejected", http.StatusNotFound)
		return
	}
	branch, err := runGit(r.Context(), record.Workspace, "branch", "--show-current")
	if err != nil || safeBranchName(branch) != nil {
		http.Error(w, "protected or invalid branch", http.StatusForbidden)
		return
	}
	if _, err := runGit(r.Context(), record.Workspace, "push", "-u", "origin", branch); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	_ = s.tasks.audit(id, "git_push", map[string]string{"branch": branch})
	writeJSON(w, map[string]string{"task_id": id, "branch": branch, "status": "pushed"})
}

func (s *Server) apiTaskPR(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.taskWorkspace(id)
	if err != nil {
		http.Error(w, "task not found or workspace rejected", http.StatusNotFound)
		return
	}
	token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if token == "" {
		http.Error(w, "GITHUB_TOKEN is not configured", http.StatusServiceUnavailable)
		return
	}
	remote, err := runGit(r.Context(), record.Workspace, "remote", "get-url", "origin")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	owner, repo, err := githubRepo(remote)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	branch, err := runGit(r.Context(), record.Workspace, "branch", "--show-current")
	if err != nil || safeBranchName(branch) != nil {
		http.Error(w, "protected or invalid branch", http.StatusForbidden)
		return
	}
	var body struct{ Title, Body, Base string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid pull request body", http.StatusBadRequest)
		return
	}
	if body.Title == "" {
		body.Title = "Vcode task " + id
	}
	if body.Base == "" {
		body.Base = "main"
	}
	payload := map[string]string{"title": body.Title, "body": body.Body, "head": branch, "base": body.Base}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, "https://api.github.com/repos/"+owner+"/"+repo+"/pulls", strings.NewReader(string(data)))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "GitHub request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	result, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		http.Error(w, redactGitOutput(string(result)), resp.StatusCode)
		return
	}
	_ = s.tasks.audit(id, "git_pr", map[string]string{"owner": owner, "repo": repo, "branch": branch})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(result)
}

func githubRepo(raw string) (string, string, error) {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ".git")
	if strings.HasPrefix(raw, "git@github.com:") {
		raw = strings.TrimPrefix(raw, "git@github.com:")
	} else if u, err := url.Parse(raw); err == nil && u.Host == "github.com" {
		raw = strings.TrimPrefix(u.Path, "/")
	} else {
		return "", "", errors.New("origin is not a GitHub repository")
	}
	parts := strings.Split(strings.Trim(raw, "/"), "/")
	if len(parts) != 2 || !projectIDPattern.MatchString(parts[0]) || !projectIDPattern.MatchString(parts[1]) {
		return "", "", errors.New("invalid GitHub repository")
	}
	return parts[0], parts[1], nil
}

func (s *Server) apiTaskControl(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.tasks.record(id)
	if err != nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid control body", http.StatusBadRequest)
		return
	}
	switch strings.ToLower(strings.TrimSpace(body.Action)) {
	case "pause", "cancel":
		s.ctl().Cancel()
		status := TaskPaused
		if strings.EqualFold(body.Action, "cancel") {
			status = TaskCancelled
		}
		_ = s.tasks.update(id, status, "user_cancelled", "任务被用户暂停")
	case "continue", "retry":
		_ = s.tasks.update(id, TaskQueued, "", "")
		s.bc.SetActiveTask(id)
		s.ctl().SubmitHTTP("继续执行任务：" + record.Goal)
	default:
		http.Error(w, "action must be pause, continue, retry, or cancel", http.StatusBadRequest)
		return
	}
	_ = s.tasks.audit(id, "task_control", map[string]string{"action": strings.ToLower(strings.TrimSpace(body.Action))})
	writeJSON(w, map[string]string{"task_id": id, "status": string(s.tasks.activeRecord().Status)})
}
