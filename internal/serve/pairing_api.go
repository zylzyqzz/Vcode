package serve

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"vcode/internal/runtime"
)

var (
	errTargetOffline  = errors.New("target is offline")
	errPairingExpired = errors.New("pairing code expired")
	errPairingInvalid = errors.New("invalid or already used pairing code")
)

// apiBridgePair is called by the local bridge after `vcode bridge pair`.
// It is authenticated with the existing bridge token; the token is never
// returned to the browser or included in the pairing URL.
func (s *Server) apiBridgePair(w http.ResponseWriter, r *http.Request) {
	if s.relay == nil || !s.relay.authorized(r.Header.Get("X-Vcode-Bridge-Token")) {
		http.Error(w, "bridge unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		TargetID  string    `json:"target_id"`
		Code      string    `json:"code"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid pairing request", http.StatusBadRequest)
		return
	}
	if err := s.relay.publishPairing(runtime.PairingRequest{TargetID: strings.TrimSpace(body.TargetID), Code: strings.TrimSpace(body.Code), ExpiresAt: body.ExpiresAt}); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]string{"status": "pending"})
}

func (r *bridgeRelay) publishPairing(p runtime.PairingRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	client := r.clients[p.TargetID]
	if client == nil {
		return errTargetOffline
	}
	if p.Code == "" || p.ExpiresAt.Before(time.Now()) {
		return errPairingExpired
	}
	client.pairing = &p
	return nil
}

func (r *bridgeRelay) approvePairing(targetID, code string) (runtime.RuntimeTarget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client := r.clients[strings.TrimSpace(targetID)]
	if client == nil {
		return runtime.RuntimeTarget{}, errTargetOffline
	}
	if client.pairing == nil || client.pairing.Used || time.Now().After(client.pairing.ExpiresAt) || client.pairing.Code != strings.TrimSpace(code) {
		return runtime.RuntimeTarget{}, errPairingInvalid
	}
	client.pairing.Used = true
	return client.target, nil
}

func (s *Server) apiPairApprove(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetID string `json:"target_id"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid pairing request", http.StatusBadRequest)
		return
	}
	target, err := s.relay.approvePairing(body.TargetID, body.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, map[string]any{"status": "approved", "target": target})
}

func (s *Server) pairingPage(w http.ResponseWriter, _ *http.Request) {
	// The page is deliberately tiny: opening a QR/deep link approves the
	// already-authenticated phone session, then returns to the terminal UI.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(pairingPageHTML))
}

const pairingPageHTML = `<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>Vcode 连接</title><style>body{margin:0;background:#0d0d0d;color:#eee;font:16px system-ui;padding:32px}main{max-width:420px;margin:12vh auto;border:1px solid #b18a28;border-radius:16px;padding:24px;box-shadow:0 0 28px #b18a2833}h1{color:#d2a83a}p{color:#aaa;line-height:1.6}button{background:#b18a28;color:#111;border:0;border-radius:9px;padding:12px 18px;font-weight:700}</style><main><h1>Vcode</h1><p id="status">正在验证这台电脑…</p><button onclick="approve()">重新连接</button></main><script>const q=new URLSearchParams(location.search);async function approve(){const r=await fetch('/api/targets/pair/approve',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({target_id:q.get('target'),code:q.get('code')})});const t=await r.text();document.querySelector('#status').textContent=r.ok?'连接成功，正在返回 Vcode。':('连接失败：'+t);if(r.ok)setTimeout(()=>location.href='/?target='+encodeURIComponent(q.get('target')||''),500)}approve();</script>`
