package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRemovalBoundaries(t *testing.T) {
	bad := []string{"/", "/opt/harbor", "/etc/harbor/control.env", "/var/lib/harbor", "/etc/systemd/system", "/opt/harbor-agent/../../etc", "/opt/harbor-agent-other"}
	for _, p := range bad {
		if safePath(p) {
			t.Fatalf("dangerous removal allowed: %s", p)
		}
	}
	if runtime.GOOS == "windows" {
		if !safePath(filepath.Join(os.Getenv("ProgramData"), "StarhubAgent", "runtime")) {
			t.Fatal("Windows installation refused")
		}
	} else if !safePath("/var/lib/harbor-agent/live") {
		t.Fatal("Agent state refused")
	}
}

func TestOwnedAgentUnits(t *testing.T) {
	for _, owned := range []string{"nodehub-agent.service", "harbor-agent.service"} {
		t.Run(owned, func(t *testing.T) {
			dir := t.TempDir()
			env := "/etc/harbor-agent/agent.env"
			for _, name := range []string{"nodehub-agent.service", "harbor-agent.service"} {
				unitEnv := "/etc/other.env"
				if name == owned {
					unitEnv = env
				}
				if err := os.WriteFile(filepath.Join(dir, name), []byte("EnvironmentFile="+unitEnv+"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			units, err := ownedAgentUnits(dir, env)
			if err != nil || len(units) != 1 || filepath.Base(units[0]) != owned {
				t.Fatalf("owned units = %#v, err=%v", units, err)
			}
		})
	}
}
