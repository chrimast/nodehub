package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestAgentInboundAndClientMutationsMarkDirty(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	node := &model.Node{Name: "agent-crud", NodeType: model.NodeTypeAgent, Address: "127.0.0.1", Port: 2096, ApiToken: "token", Enable: true, Status: "offline"}
	if err := db.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	inboundSvc, clientSvc := &InboundService{}, &ClientService{}
	checkDirty := func(stage string) {
		t.Helper()
		var stored model.Node
		if err := db.First(&stored, node.Id).Error; err != nil {
			t.Fatal(err)
		}
		if !stored.ConfigDirty || stored.ConfigDirtyAt <= 0 {
			t.Fatalf("%s: dirty=%v dirtyAt=%d", stage, stored.ConfigDirty, stored.ConfigDirtyAt)
		}
		if err := (&NodeService{}).ClearNodeDirty(node.Id, stored.ConfigDirtyAt); err != nil {
			t.Fatal(err)
		}
	}
	ib := &model.Inbound{UserId: 1, NodeID: &node.Id, Enable: true, Port: 24443, Protocol: model.VLESS,
		Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp","security":"none"}`}
	created, _, err := inboundSvc.AddInbound(ib)
	if err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	checkDirty("create inbound")
	created.Remark = "changed"
	if _, _, err := inboundSvc.UpdateInbound(created); err != nil {
		t.Fatalf("update inbound: %v", err)
	}
	checkDirty("update inbound")
	client := model.Client{ID: "11111111-2222-4333-8444-555555555555", Email: "agent-client@example.com", Enable: true}
	if _, err := clientSvc.AddInboundClient(inboundSvc, &model.Inbound{Id: created.Id, Settings: clientsSettings(t, []model.Client{client})}); err != nil {
		t.Fatalf("add client: %v", err)
	}
	checkDirty("add client")
	client.Email = "agent-client-new@example.com"
	if _, err := clientSvc.UpdateInboundClient(inboundSvc, &model.Inbound{Id: created.Id, Settings: clientsSettings(t, []model.Client{client})}, "agent-client@example.com"); err != nil {
		t.Fatalf("update client: %v", err)
	}
	checkDirty("update client")
	if _, err := clientSvc.DelInboundClientByEmail(inboundSvc, created.Id, client.Email, false, false); err != nil {
		t.Fatalf("delete client: %v", err)
	}
	checkDirty("delete client")
	if _, err := inboundSvc.DelInbound(created.Id); err != nil {
		t.Fatalf("delete inbound: %v", err)
	}
	checkDirty("delete inbound")
}
