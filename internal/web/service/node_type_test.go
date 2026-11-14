package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestNodeTypeDefaultsToPanelAndIsReturnedByNodeView(t *testing.T) {
	setupConflictDB(t)
	token := "panel-token"
	view, err := (&NodeService{}).CreateFromRequest(&NodeMutationRequest{
		Name: "default-panel", Scheme: "https", Address: "127.0.0.1", Port: 2096,
		ApiToken: &token, Enable: true,
	})
	if err != nil {
		t.Fatalf("create panel node: %v", err)
	}
	if view.NodeType != string(model.NodeTypePanel) {
		t.Fatalf("NodeType = %q, want %q", view.NodeType, model.NodeTypePanel)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, view.Id).Error; err != nil {
		t.Fatalf("load node: %v", err)
	}
	if stored.NodeType != model.NodeTypePanel {
		t.Fatalf("stored NodeType = %q, want panel", stored.NodeType)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"nodeType":"panel"`) {
		t.Fatalf("API view omitted nodeType: %s", raw)
	}
}

func TestNodeTypeAgentPersistsAndInvalidTypeIsRejected(t *testing.T) {
	setupConflictDB(t)
	token := "agent-token"
	view, err := (&NodeService{}).CreateFromRequest(&NodeMutationRequest{
		Name: "agent-node", NodeType: string(model.NodeTypeAgent), Scheme: "https", Address: "127.0.0.1", Port: 2097,
		ApiToken: &token, Enable: true,
	})
	if err != nil {
		t.Fatalf("create agent node: %v", err)
	}
	if view.NodeType != string(model.NodeTypeAgent) {
		t.Fatalf("NodeType = %q, want agent", view.NodeType)
	}
	var stored model.Node
	if err := database.GetDB().First(&stored, view.Id).Error; err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if stored.NodeType != model.NodeTypeAgent {
		t.Fatalf("stored NodeType = %q, want agent", stored.NodeType)
	}

	bad := "bad-type"
	_, err = (&NodeService{}).CreateFromRequest(&NodeMutationRequest{
		Name: "invalid-node", NodeType: bad, Scheme: "https", Address: "127.0.0.1", Port: 2098,
		ApiToken: &token, Enable: true,
	})
	if err == nil || !strings.Contains(err.Error(), "nodeType") {
		t.Fatalf("invalid node type error = %v, want nodeType validation error", err)
	}
}
