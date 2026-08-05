package serve

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"vcode/internal/config"
	"vcode/internal/control"
)

func TestTargetRoutesAreRegistered(t *testing.T) {
	bc := NewBroadcaster()
	ctrl := control.New(control.Options{Runner: fakeRunner{got: make(chan string, 1)}, Sink: bc})
	srv := httptest.NewServer(New(ctrl, bc, config.ServeConfig{}).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/targets")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	var envelope struct {
		Targets []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Targets) != 1 || envelope.Targets[0].ID != "cloud" || envelope.Targets[0].Kind != "cloud" {
		t.Fatalf("targets = %+v", envelope.Targets)
	}
}
