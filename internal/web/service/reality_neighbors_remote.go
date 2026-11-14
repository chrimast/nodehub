package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// ScanRealityNeighborsForNode delegates to an authenticated node panel when
// nodeID is set. The node always chooses its own cached public IPv4.
func (s *NodeService) ScanRealityNeighborsForNode(ctx context.Context, nodeID, limit int) ([]*RealityScanResult, int, error) {
	if nodeID <= 0 {
		return nil, 0, errors.New("remote REALITY neighbor scan requires a node")
	}
	node, err := s.GetById(nodeID)
	if err != nil {
		return nil, 0, err
	}
	if err := s.normalize(node); err != nil {
		return nil, 0, err
	}
	raw, err := runtime.NewRemote(node, nil).ScanRealityNeighbors(ctx, limit)
	if err != nil {
		return nil, 0, err
	}
	var response struct {
		Results []*RealityScanResult `json:"results"`
		Scanned int                  `json:"scanned"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, 0, fmt.Errorf("decode remote REALITY neighbor scan response: %w", err)
	}
	return response.Results, response.Scanned, nil
}
