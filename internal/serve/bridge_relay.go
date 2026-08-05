package serve

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"vcode/internal/bridge"
	"vcode/internal/runtime"
)

type bridgeRelay struct {
	mu      sync.RWMutex
	token   string
	clients map[string]*bridgeClient
	events  map[string][]bridge.Message
	tasks   map[string]*TaskRecord
	seq     map[string]uint64
	onEvent func(bridge.Message)
}

type bridgeClient struct {
	conn    *websocket.Conn
	write   sync.Mutex
	target  runtime.RuntimeTarget
	pairing *runtime.PairingRequest
}

func newBridgeRelay(token string) *bridgeRelay {
	if strings.TrimSpace(token) == "" {
		token = strings.TrimSpace(os.Getenv("VCODE_BRIDGE_TOKEN"))
	}
	return &bridgeRelay{token: token, clients: make(map[string]*bridgeClient), events: make(map[string][]bridge.Message), tasks: make(map[string]*TaskRecord), seq: make(map[string]uint64)}
}

func (r *bridgeRelay) authorized(got string) bool {
	if r == nil || r.token == "" || got == "" {
		return false
	}
	if len(got) != len(r.token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(r.token)) == 1
}

func (r *bridgeRelay) snapshot() []runtime.RuntimeTarget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]runtime.RuntimeTarget, 0, len(r.clients))
	for _, client := range r.clients {
		t := client.target
		result = append(result, t)
	}
	return result
}

func (r *bridgeRelay) client(id string) *bridgeClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[id]
}

func (r *bridgeRelay) replace(client *bridgeClient) {
	r.mu.Lock()
	old := r.clients[client.target.ID]
	r.clients[client.target.ID] = client
	r.mu.Unlock()
	if old != nil && old != client {
		_ = old.conn.Close()
	}
}

func (r *bridgeRelay) remove(id string, client *bridgeClient) {
	r.mu.Lock()
	if current := r.clients[id]; current == client {
		delete(r.clients, id)
	}
	r.mu.Unlock()
}

func (r *bridgeRelay) send(id string, message bridge.Message) error {
	client := r.client(id)
	if client == nil {
		return errors.New("target is offline")
	}
	client.write.Lock()
	defer client.write.Unlock()
	return client.conn.WriteJSON(message)
}

func (r *bridgeRelay) record(message bridge.Message) {
	r.mu.Lock()
	r.seq[message.TaskID]++
	message.Seq = r.seq[message.TaskID]
	items := append(r.events[message.TaskID], message)
	if len(items) > 256 {
		items = items[len(items)-256:]
	}
	r.events[message.TaskID] = items
	if task := r.tasks[message.TaskID]; task != nil {
		task.LastEvent = message.Seq
		task.UpdatedAt = time.Now().UTC()
		var payload struct {
			Type  string `json:"type"`
			Error string `json:"error,omitempty"`
		}
		if json.Unmarshal(message.Payload, &payload) == nil {
			switch payload.Type {
			case "task_started":
				task.Status = TaskRunning
			case "task_completed":
				task.Status = TaskCompleted
				now := time.Now().UTC()
				task.FinishedAt = &now
			case "task_failed":
				task.Status = TaskFailed
				task.Error = payload.Error
				now := time.Now().UTC()
				task.FinishedAt = &now
			}
		}
	}
	onEvent := r.onEvent
	r.mu.Unlock()
	if onEvent != nil {
		onEvent(message)
	}
}

func (r *bridgeRelay) eventsAfter(taskID string, after uint64) ([]bridge.Message, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, known := r.tasks[taskID]
	items := r.events[taskID]
	result := make([]bridge.Message, 0, len(items))
	for _, item := range items {
		if item.Seq > after {
			result = append(result, item)
		}
	}
	return result, known
}

func (r *bridgeRelay) addTask(task *TaskRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
}

func (r *bridgeRelay) task(id string) (*TaskRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[id]
	if !ok {
		return nil, false
	}
	copy := *task
	return &copy, true
}

func (r *bridgeRelay) taskList() []TaskRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]TaskRecord, 0, len(r.tasks))
	for _, task := range r.tasks {
		result = append(result, *task)
	}
	return result
}

var bridgeUpgrader = websocket.Upgrader{
	ReadBufferSize:  16 << 10,
	WriteBufferSize: 16 << 10,
	CheckOrigin:     func(*http.Request) bool { return true }, // authenticated by BridgeToken
}

func (s *Server) bridgeConnect(w http.ResponseWriter, req *http.Request) {
	token := req.Header.Get("X-Vcode-Bridge-Token")
	if token == "" {
		token = req.URL.Query().Get("token")
	}
	if !s.relay.authorized(token) {
		slog.Warn("bridge relay unauthorized")
		http.Error(w, "bridge unauthorized", http.StatusUnauthorized)
		return
	}
	conn, err := bridgeUpgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Warn("bridge relay upgrade failed", "err", err)
		return
	}
	client := &bridgeClient{conn: conn}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var hello bridge.Message
	if err := conn.ReadJSON(&hello); err != nil || hello.Type != bridge.MessageHello {
		slog.Warn("bridge relay hello failed", "err", err)
		return
	}
	var helloPayload bridge.HelloPayload
	if err := json.Unmarshal(hello.Payload, &helloPayload); err != nil || helloPayload.Target.ID == "" {
		// Backward compatibility for bridges that still send the original flat
		// RuntimeTarget payload.
		_ = json.Unmarshal(hello.Payload, &helloPayload.Target)
	}
	target := helloPayload.Target
	if target.ID == "" || target.Kind != runtime.TargetLocalComputer {
		slog.Warn("bridge relay invalid target", "target_id", target.ID)
		return
	}
	client.pairing = helloPayload.Pairing
	client.target = target
	client.target.Status = runtime.TargetOnline
	client.target.LastSeen = time.Now().UTC()
	s.relay.replace(client)
	defer s.relay.remove(target.ID, client)
	conn.SetReadDeadline(time.Time{})
	_ = client.writeJSON(bridge.Message{Type: bridge.MessageHelloAck, TargetID: target.ID, Timestamp: time.Now().UTC()})
	for {
		var message bridge.Message
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		if message.Type == bridge.MessageRuntimeEvent {
			s.relay.record(message)
			// The task journal/SSE path is the next consumer; for now keep the
			// relay transparent and update liveness from every valid event.
			s.relay.mu.Lock()
			if current := s.relay.clients[target.ID]; current == client {
				current.target.LastSeen = time.Now().UTC()
			}
			s.relay.mu.Unlock()
		}
	}
}

func (c *bridgeClient) writeJSON(message bridge.Message) error {
	c.write.Lock()
	defer c.write.Unlock()
	return c.conn.WriteJSON(message)
}

func (s *Server) apiTargetTask(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" || id == "cloud" {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Input   string `json:"input"`
		Mode    string `json:"mode,omitempty"`
		Project string `json:"project,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Input) == "" {
		http.Error(w, "input is required", http.StatusBadRequest)
		return
	}
	taskID := "remote-" + randomID()
	payload, _ := json.Marshal(map[string]string{"input": body.Input, "mode": body.Mode, "project": body.Project})
	now := time.Now().UTC()
	s.relay.addTask(&TaskRecord{ID: taskID, Goal: body.Input, Mode: body.Mode, Status: TaskQueued, CreatedAt: now, UpdatedAt: now})
	if err := s.relay.send(id, bridge.Message{Type: bridge.MessageTaskSubmit, RequestID: taskID, TargetID: id, TaskID: taskID, Timestamp: time.Now().UTC(), Payload: payload}); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSONStatus(w, http.StatusAccepted, map[string]any{"task_id": taskID, "target_id": id, "status": runtime.TaskQueued})
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "task"
	}
	return hex.EncodeToString(b)
}
