package service

import (
	"encoding/json"
	"errors"
	"io"
)

// AgentCapabilities reports advertised operations, not actual Xray state.
type AgentCapabilities struct {
	Version       int  `json:"version"`
	ConfigSave    bool `json:"config_save"`
	ConfigApply   bool `json:"config_apply"`
	ConfigDisable bool `json:"config_disable"`
	ConfigVersion bool `json:"config_version"`
	RuntimeHealth bool `json:"runtime_health"`
}

// decodeAgentCapabilities validates the version and all required flags.
// Call only on an authenticated 200 response using the node's normal transport.
func decodeAgentCapabilities(r io.Reader) (AgentCapabilities, error) {
	var result AgentCapabilities
	b, err := io.ReadAll(io.LimitReader(r, 65537))
	if err != nil || len(b) > 65536 {
		return result, errors.New("invalid agent capabilities response")
	}
	var wire struct {
		Version       *int  `json:"version"`
		ConfigSave    *bool `json:"config_save"`
		ConfigApply   *bool `json:"config_apply"`
		ConfigDisable *bool `json:"config_disable"`
		ConfigVersion *bool `json:"config_version"`
		RuntimeHealth *bool `json:"runtime_health"`
	}
	if json.Unmarshal(b, &wire) != nil || wire.Version == nil || *wire.Version != 1 || wire.ConfigSave == nil || wire.ConfigApply == nil || wire.RuntimeHealth == nil {
		return result, errors.New("invalid or unsupported agent capabilities response")
	}
	return AgentCapabilities{Version: *wire.Version, ConfigSave: *wire.ConfigSave, ConfigApply: *wire.ConfigApply, ConfigDisable: wire.ConfigDisable != nil && *wire.ConfigDisable, ConfigVersion: wire.ConfigVersion != nil && *wire.ConfigVersion, RuntimeHealth: *wire.RuntimeHealth}, nil
}
