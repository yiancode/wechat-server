package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestForwarderHeadersUnmarshal(t *testing.T) {
	data := []byte(`
app_id: "wx123"
token: "tok"
name: "acc"
forwarders:
  - name: "code80"
    url: "https://code.example.com/api/v1/webhook/wechat/mp"
    priority: 1
    events: ["subscribe", "SCAN"]
    timeout: 5000
    headers:
      X-Internal-Forward-Token: "secret-value"
  - name: "no-headers"
    url: "https://other.example.com/hook"
`)

	var acc WechatAccount
	if err := yaml.Unmarshal(data, &acc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(acc.Forwarders) != 2 {
		t.Fatalf("expected 2 forwarders, got %d", len(acc.Forwarders))
	}
	if got := acc.Forwarders[0].Headers["X-Internal-Forward-Token"]; got != "secret-value" {
		t.Fatalf("expected header secret-value, got %q", got)
	}
	if acc.Forwarders[1].Headers != nil {
		t.Fatalf("expected nil headers for forwarder without headers, got %v", acc.Forwarders[1].Headers)
	}
}
