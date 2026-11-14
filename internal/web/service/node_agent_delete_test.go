package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestDeleteAgentPreservesPendingLastInboundRemoval(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	node := &model.Node{Name: "agent", NodeType: model.NodeTypeAgent, Address: "127.0.0.1", Port: 2096, ApiToken: "token", Enable: true, Status: "offline"}
	if err := db.Create(node).Error; err != nil { t.Fatal(err) }
	ib := &model.Inbound{UserId: 1, NodeID: &node.Id, Enable: true, Port: 24443, Protocol: model.VLESS, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"tcp","security":"none"}`}
	created, _, err := (&InboundService{}).AddInbound(ib)
	if err != nil { t.Fatal(err) }
	if _, err := (&InboundService{}).DelInbound(created.Id); err != nil { t.Fatal(err) }
	if err := (&NodeService{}).SetEnable(node.Id, false); err != nil { t.Fatal(err) }
	err = (&NodeService{}).Delete(node.Id)
	if err == nil || !strings.Contains(err.Error(), "unsynchronized") { t.Fatalf("expected unsynchronized config error, got %v", err) }
	var stored model.Node
	if err := db.First(&stored, node.Id).Error; err != nil { t.Fatalf("node must remain: %v", err) }
	if !stored.ConfigDirty { t.Fatal("pending remote removal must remain dirty") }
	var count int64
	if err := db.Model(&model.Inbound{}).Where("node_id = ?", node.Id).Count(&count).Error; err != nil || count != 0 { t.Fatalf("inbound count=%d err=%v", count, err) }
	if err := (&NodeService{}).ClearNodeDirty(node.Id, stored.ConfigDirtyAt); err != nil { t.Fatal(err) }
	if err := (&NodeService{}).Delete(node.Id); err != nil { t.Fatalf("clean agent deletion: %v", err) }
	if err := db.First(&stored, node.Id).Error; err == nil { t.Fatal("clean agent was not deleted") }
}

func TestDeleteDirtyPanelRemainsAllowed(t *testing.T) {
	setupConflictDB(t)
	node := &model.Node{Name: "panel", NodeType: model.NodeTypePanel, Address: "127.0.0.1", Port: 2096, ConfigDirty: true}
	if err := database.GetDB().Create(node).Error; err != nil { t.Fatal(err) }
	if err := (&NodeService{}).Delete(node.Id); err != nil { t.Fatalf("delete panel: %v", err) }
	var count int64
	if err := database.GetDB().Model(&model.Node{}).Where("id = ?", node.Id).Count(&count).Error; err != nil || count != 0 { t.Fatalf("node count=%d err=%v", count, err) }
}