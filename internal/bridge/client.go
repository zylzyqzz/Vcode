package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"vcode/internal/runtime"
)

// Client is the outbound computer-side relay client. It only accepts the
// structured task_submit message; it never turns a phone-provided string into
// a shell command.
type Client struct {
	RelayURL string
	Token    string
	Store    *Store
	Output   func(Message)
	mu       sync.Mutex
	active   map[string]context.CancelFunc
}

func (c *Client) Run(ctx context.Context) error {
	if c == nil || c.Store == nil || strings.TrimSpace(c.RelayURL) == "" {
		return errors.New("bridge relay URL and store are required")
	}
	attempt := 0
	if c.active == nil {
		c.active = make(map[string]context.CancelFunc)
	}
	for {
		err := c.connect(ctx)
		if err == nil || ctx.Err() != nil {
			return err
		}
		if c.Output != nil {
			c.Output(Message{Type: MessageError, Timestamp: time.Now().UTC(), Payload: json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error()))})
		}
		_ = c.Store.SetStatus(runtime.TargetOffline)
		attempt++
		delay := RetryDelay(attempt - 1)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Client) connect(ctx context.Context) error {
	header := make(http.Header)
	header.Set("X-Vcode-Bridge-Token", c.Token)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.RelayURL, header)
	if err != nil {
		return err
	}
	defer conn.Close()
	target := c.Store.Snapshot()
	payload, _ := json.Marshal(HelloPayload{Target: target, Pairing: c.Store.PairingSnapshot()})
	if err := conn.WriteJSON(Message{Type: MessageHello, TargetID: target.ID, Timestamp: time.Now().UTC(), Payload: payload}); err != nil {
		return err
	}
	if err := c.Store.SetStatus(runtime.TargetOnline); err != nil {
		return err
	}
	for {
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			_ = c.Store.SetStatus(runtime.TargetOffline)
			return err
		}
		switch message.Type {
		case MessageTaskSubmit:
			go c.handleTask(ctx, conn, message)
		case MessageTaskControl:
			c.controlTask(message)
		}
	}
}

func (c *Client) handleTask(parent context.Context, conn *websocket.Conn, message Message) {
	taskCtx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.active[message.TaskID] = cancel
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		delete(c.active, message.TaskID)
		c.mu.Unlock()
	}()
	var body struct {
		Input   string `json:"input"`
		Mode    string `json:"mode"`
		Project string `json:"project"`
	}
	if err := json.Unmarshal(message.Payload, &body); err != nil || strings.TrimSpace(body.Input) == "" {
		c.emit(conn, message, "task_failed", map[string]string{"error": "invalid task payload"})
		return
	}
	project, ok := c.project(body.Project)
	if !ok {
		c.emit(conn, message, "task_failed", map[string]string{"error": "project is not registered on this computer"})
		return
	}
	c.emit(conn, message, "task_started", map[string]string{"project": project.ID, "mode": body.Mode})
	// The non-interactive run command does not expose the interactive --yolo
	// flag. Build/Goal approval policy is configured by the local Vcode config;
	// passing --yolo here made every computer task fail with exit status 2.
	args := []string{"run", "--dir", project.Root, body.Input}
	cmd := exec.CommandContext(taskCtx, os.Args[0], args...)
	out, err := cmd.CombinedOutput()
	status := "task_completed"
	if err != nil {
		status = "task_failed"
	}
	c.emit(conn, message, status, map[string]string{"output": string(out), "error": errorText(err)})
}

func (c *Client) controlTask(message Message) {
	var body struct {
		Action string `json:"action"`
	}
	if json.Unmarshal(message.Payload, &body) != nil {
		return
	}
	if body.Action != "cancel" && body.Action != "pause" {
		return
	}
	c.mu.Lock()
	cancel := c.active[message.TaskID]
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *Client) project(id string) (runtime.LocalProject, bool) {
	projects := c.Store.ProjectsSnapshot()
	for _, project := range projects {
		if id == "" || project.ID == id || project.Name == id {
			return project, true
		}
	}
	return runtime.LocalProject{}, false
}

func (c *Client) emit(conn *websocket.Conn, task Message, typ string, value map[string]string) {
	value["type"] = typ
	payload, _ := json.Marshal(value)
	message := Message{Type: MessageRuntimeEvent, TargetID: task.TargetID, TaskID: task.TaskID, RequestID: task.RequestID, Timestamp: time.Now().UTC(), Payload: payload}
	if c.Output != nil {
		c.Output(message)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = conn.WriteJSON(message)
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}
