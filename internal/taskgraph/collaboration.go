package taskgraph

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// AgentMessage is a durable mailbox entry. Messages survive process restarts
// so a resumed Coordinator can reconstruct why an Agent was reassigned.
type AgentMessage struct {
	ID          string     `json:"id"`
	From        string     `json:"from"`
	To          string     `json:"to"`
	Kind        string     `json:"kind"`
	Body        string     `json:"body"`
	ReplyTo     string     `json:"reply_to,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

type AgentPresence struct {
	AgentID  string    `json:"agent_id"`
	NodeID   string    `json:"node_id,omitempty"`
	Role     Role      `json:"role,omitempty"`
	State    string    `json:"state"`
	LastSeen time.Time `json:"last_seen"`
	Error    string    `json:"error,omitempty"`
}

func (s *Store) SendMessage(t *Task, message AgentMessage) error {
	if t == nil {
		return errors.New("task is nil")
	}
	message.From = strings.TrimSpace(message.From)
	message.To = strings.TrimSpace(message.To)
	message.Kind = strings.TrimSpace(message.Kind)
	message.Body = strings.TrimSpace(message.Body)
	if message.From == "" || message.To == "" || message.Kind == "" || message.Body == "" {
		return errors.New("message from, to, kind, and body are required")
	}
	if message.ID == "" {
		message.ID = fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), len(t.Messages)+1)
	}
	message.CreatedAt = time.Now().UTC()
	t.Messages = append(t.Messages, message)
	return s.AppendEvent(t, Event{Type: "agent_message", AgentID: message.From, ParentID: message.ReplyTo, Message: message.Kind, Data: map[string]string{
		"to": message.To, "message_id": message.ID,
	}})
}

func (t Task) PendingMessages(agentID string) []AgentMessage {
	agentID = strings.TrimSpace(agentID)
	var out []AgentMessage
	for _, message := range t.Messages {
		if message.DeliveredAt == nil && (message.To == "*" || message.To == agentID) {
			out = append(out, message)
		}
	}
	return out
}

func (s *Store) MarkMessagesDelivered(t *Task, agentID string, ids ...string) error {
	if t == nil {
		return errors.New("task is nil")
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[strings.TrimSpace(id)] = true
	}
	now := time.Now().UTC()
	for i := range t.Messages {
		m := &t.Messages[i]
		if m.DeliveredAt == nil && (m.To == "*" || m.To == agentID) && wanted[m.ID] {
			stamp := now
			m.DeliveredAt = &stamp
		}
	}
	return s.Save(*t)
}

func (s *Store) Heartbeat(t *Task, presence AgentPresence) error {
	if t == nil {
		return errors.New("task is nil")
	}
	presence.AgentID = strings.TrimSpace(presence.AgentID)
	presence.State = strings.TrimSpace(presence.State)
	if presence.AgentID == "" || presence.State == "" {
		return errors.New("agent heartbeat requires agent_id and state")
	}
	presence.LastSeen = time.Now().UTC()
	for i := range t.Agents {
		if t.Agents[i].AgentID == presence.AgentID {
			t.Agents[i] = presence
			return s.Save(*t)
		}
	}
	t.Agents = append(t.Agents, presence)
	return s.AppendEvent(t, Event{Type: "agent_heartbeat", AgentID: presence.AgentID, NodeID: presence.NodeID, Role: presence.Role, Message: presence.State})
}
