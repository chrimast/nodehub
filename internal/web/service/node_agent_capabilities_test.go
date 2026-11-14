package service

import (
	"strings"
	"testing"
)

func TestDecodeAgentCapabilities(t *testing.T) {
	valid := `{"version":1,"config_save":true,"config_apply":false,"runtime_health":false}`
	got, err := decodeAgentCapabilities(strings.NewReader(valid))
	if err != nil || !got.ConfigSave || got.ConfigApply || got.RuntimeHealth {
		t.Fatalf("got %+v, %v", got, err)
	}
	for _, body := range []string{`{}`, `{"version":2,"config_save":true,"config_apply":false,"runtime_health":false}`, `{"version":1}`, valid + ` {}`, strings.Repeat(" ", 65537) + valid} {
		if _, err := decodeAgentCapabilities(strings.NewReader(body)); err == nil {
			t.Errorf("accepted invalid capabilities %.80q", body)
		}
	}
}
