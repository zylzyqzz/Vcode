package bridge

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"vcode/internal/runtime"
)

// PairURL is intentionally a short-lived deep link. It contains only the
// target and one-time code, never the bridge token or a model credential.
func PairURL(relayURL string, p runtime.PairingRequest) string {
	base := strings.TrimSpace(relayURL)
	base = strings.Replace(base, "wss://", "https://", 1)
	base = strings.Replace(base, "ws://", "http://", 1)
	if i := strings.Index(base, "/api/bridge/connect"); i >= 0 {
		base = base[:i]
	}
	if base == "" {
		base = "https://v.aimj.xin"
	}
	return strings.TrimRight(base, "/") + "/pair?target=" + url.QueryEscape(p.TargetID) + "&code=" + url.QueryEscape(p.Code)
}

// PublishPairing lets a currently connected bridge advertise a newly created
// local pairing request without restarting the background Bridge process.
// Existing token-based configuration remains the authentication mechanism.
func PublishPairing(relayURL, token string, p runtime.PairingRequest) error {
	if strings.TrimSpace(relayURL) == "" || strings.TrimSpace(token) == "" {
		return errors.New("bridge relay is not configured")
	}
	base := strings.Replace(strings.TrimSpace(relayURL), "wss://", "https://", 1)
	base = strings.Replace(base, "ws://", "http://", 1)
	if i := strings.Index(base, "/api/bridge/connect"); i >= 0 {
		base = base[:i]
	}
	body, _ := json.Marshal(map[string]any{"target_id": p.TargetID, "code": p.Code, "expires_at": p.ExpiresAt})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/api/bridge/pair", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Vcode-Bridge-Token", token)
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("relay rejected pairing: %s", resp.Status)
	}
	return nil
}
