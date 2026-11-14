package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/netsafe"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// AgentApplyResult confirms activation only when the agent explicitly acknowledges it.
type AgentApplyResult struct {
	Applied    bool   `json:"applied"`
	Status     string `json:"status"`
	Generation int64  `json:"-"`
}

// GetAgentXrayConfig deliberately uses direct egress and no panel template or sidecars.
// Only protocols whose client representation is handled here are accepted.
func (s *XrayService) GetAgentXrayConfig(nodeID int) (map[string]any, error) {
	if nodeID <= 0 {
		return nil, errors.New("invalid node ID")
	}
	var inbounds []*model.Inbound
	db := database.GetDB()
	if err := db.Where("node_id = ? AND enable = ?", nodeID, true).Order("id").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	if len(inbounds) == 0 {
		return map[string]any{"inbounds": []any{}, "outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}}}, nil
	}
	if err := attachClientStats(db, inbounds); err != nil {
		return nil, err
	}
	configs := make([]xray.InboundConfig, 0, len(inbounds))
	for _, in := range inbounds {
		if in.Total != 0 || in.ExpiryTime != 0 {
			return nil, errors.New("agent cannot enforce inbound traffic or expiry limits")
		}
		switch in.Protocol {
		case model.VLESS, model.VMESS, model.Trojan, model.Shadowsocks:
		default:
			return nil, fmt.Errorf("unsupported agent protocol %s", in.Protocol)
		}
		fallbackRows, err := (&FallbackService{}).GetByMaster(in.Id)
		if err != nil {
			return nil, err
		}
		parent, err := (&FallbackService{}).GetParentForChild(in.Id)
		if err != nil {
			return nil, err
		}
		if len(fallbackRows) > 0 || parent != nil {
			return nil, errors.New("managed fallbacks unsupported on agent")
		}
		var settings map[string]any
		if json.Unmarshal([]byte(in.Settings), &settings) != nil || settings == nil {
			return nil, errors.New("invalid inbound settings")
		}
		if f, ok := settings["fallbacks"]; ok && f != nil {
			if a, ok := f.([]any); !ok || len(a) > 0 {
				return nil, errors.New("fallbacks unsupported on agent")
			}
		}
		clients, err := s.inboundService.clientService.ListForInbound(nil, in.Id)
		if err != nil {
			return nil, err
		}
		stats := make(map[string]xray.ClientTraffic, len(in.ClientStats))
		emails := make([]string, 0, len(clients))
		for _, c := range clients {
			emails = append(emails, c.Email)
		}
		var trafficRows []xray.ClientTraffic
		if len(emails) > 0 && db.Where("email IN ?", emails).Find(&trafficRows).Error != nil {
			return nil, errors.New("failed to load client traffic limits")
		}
		for _, stat := range trafficRows {
			stats[stat.Email] = stat
		}
		enabled := make(map[string]bool, len(in.ClientStats))
		for _, stat := range in.ClientStats {
			enabled[stat.Email] = stat.Enable
			stats[stat.Email] = stat
		}
		entries := make([]any, 0, len(clients))
		for _, c := range clients {
			if c.LimitIP != 0 {
				return nil, errors.New("agent cannot enforce client IP limits")
			}
			stat := stats[c.Email]
			if c.TotalGB != 0 || c.ExpiryTime != 0 || stat.Total != 0 || stat.ExpiryTime != 0 {
				return nil, errors.New("agent cannot enforce client traffic or expiry limits")
			}
			if !c.Enable {
				continue
			}
			if v, ok := enabled[c.Email]; ok && !v {
				continue
			}
			entry := map[string]any{"email": c.Email}
			flow := c.Flow
			if flow == "xtls-rprx-vision-udp443" {
				flow = "xtls-rprx-vision"
			}
			if in.DisableFlow || !inboundCanEnableTlsFlow(string(in.Protocol), in.StreamSettings, in.Settings) {
				flow = ""
			}
			switch in.Protocol {
			case model.VLESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if flow != "" {
					entry["flow"] = flow
				}
				if c.Reverse != nil {
					entry["reverse"] = c.Reverse
				}
			case model.VMESS:
				if c.ID != "" {
					entry["id"] = c.ID
				}
				if c.Security != "" {
					entry["security"] = c.Security
				}
			case model.Trojan:
				if c.Password != "" {
					entry["password"] = c.Password
				}
				if flow != "" {
					entry["flow"] = flow
				}
			case model.Shadowsocks:
				if c.Password != "" {
					entry["password"] = c.Password
				}
			}
			entries = append(entries, entry)
		}
		settings["clients"] = entries
		if in.Protocol == model.Shadowsocks {
			xray.LockShadowsocks2022(settings)
		}
		b, err := json.Marshal(settings)
		if err != nil {
			return nil, err
		}
		in.Settings = string(b)
		if in.StreamSettings != "" {
			var stream map[string]any
			if json.Unmarshal([]byte(in.StreamSettings), &stream) != nil || stream == nil {
				return nil, errors.New("invalid stream settings")
			}
			delete(stream, "externalProxy")
			for _, key := range []string{"tlsSettings", "realitySettings"} {
				if v, ok := stream[key].(map[string]any); ok {
					delete(v, "settings")
				}
			}
			if len(finalMaskRealityTcpMasks(stream)) > 0 {
				return nil, errors.New("incompatible REALITY finalmask")
			}
			liftXhttpSessionIDKeys(stream)
			b, err = json.Marshal(stream)
			if err != nil {
				return nil, err
			}
			in.StreamSettings = string(b)
		}
		configs = append(configs, *in.GenXrayInboundConfig())
	}
	return map[string]any{"inbounds": configs, "outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}}}, nil
}

var agentApplyLocks sync.Map

func agentApplyLock(nodeID int) *sync.Mutex {
	lock, _ := agentApplyLocks.LoadOrStore(nodeID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// ApplyAgentConfig obtains fresh capabilities before posting the scoped config.
func (s *NodeService) ApplyAgentConfig(ctx context.Context, nodeID int) (AgentApplyResult, error) {
	lock := agentApplyLock(nodeID)
	lock.Lock()
	defer lock.Unlock()
	result := AgentApplyResult{Status: "failed"}
	n, err := s.GetById(nodeID)
	if err != nil {
		return result, err
	}
	if n.NodeType != model.NodeTypeAgent || n.ApiToken == "" {
		return result, errors.New("agent with token required")
	}
	if !n.ConfigDirty {
		if err := s.MarkNodeDirty(nodeID); err != nil {
			return result, err
		}
		n, err = s.GetById(nodeID)
		if err != nil {
			return result, err
		}
	}
	if n.ConfigDirtyAt <= 0 {
		return result, errors.New("agent config version is unavailable")
	}
	result.Generation = n.ConfigDirtyAt
	var config map[string]any
	if n.Enable {
		config, err = (&XrayService{}).GetAgentXrayConfig(nodeID)
		if err != nil {
			return result, err
		}
	} else {
		// An explicit empty scope is required even when local state is disabled.
		config = map[string]any{"inbounds": []any{}, "outbounds": []any{}}
	}
	patch, err := s.Probe(ctx, n)
	if err != nil {
		return result, err
	}
	emptyScope := false
	switch inbounds := config["inbounds"].(type) {
	case []xray.InboundConfig:
		emptyScope = len(inbounds) == 0
	case []any:
		emptyScope = len(inbounds) == 0
	}
	if patch.AgentCapabilities == nil || (emptyScope && !patch.AgentCapabilities.ConfigDisable) || (!emptyScope && !patch.AgentCapabilities.ConfigApply) {
		return result, errors.New("agent does not advertise the required config operation")
	}
	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return result, err
	}
	if n.Port <= 0 || n.Port > 65535 {
		return result, errors.New("invalid node port")
	}
	scheme := n.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	path := "v1/config"
	requestBody := any(map[string]any{"config": config})
	wantStatus := "applied"
	if emptyScope {
		path = "v1/disable"
		requestBody = map[string]any{}
		wantStatus = "disabled"
	}
	if patch.AgentCapabilities.ConfigVersion {
		if emptyScope {
			requestBody = map[string]any{"config_version": n.ConfigDirtyAt}
		} else {
			requestBody = map[string]any{"config_version": n.ConfigDirtyAt, "config": config}
		}
	}
	target := url.URL{Scheme: scheme, Host: net.JoinHostPort(addr, strconv.Itoa(n.Port)), Path: normalizeBasePath(n.BasePath) + path}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return result, err
	}
	if len(body) > 1<<20 {
		return result, errors.New("agent config too large")
	}
	req, err := http.NewRequestWithContext(netsafe.ContextWithAllowPrivate(ctx, n.AllowPrivateAddress), http.MethodPost, target.String(), bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+n.ApiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client, err := runtime.HTTPClientForNode(n, "")
	if err != nil {
		return result, err
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Status = "unknown"
		return result, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		result.Status = "unknown"
		return result, fmt.Errorf("agent config returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(data) > 65536 {
		result.Status = "unknown"
		return result, errors.New("invalid agent apply response")
	}
	var acknowledgement struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &acknowledgement) != nil || acknowledgement.Status != wantStatus {
		result.Status = "unknown"
		return result, errors.New("agent did not confirm config application")
	}
	result.Applied = true
	result.Status = wantStatus
	return result, nil
}
