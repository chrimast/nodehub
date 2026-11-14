package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// capabilityHandler advertises supported operations separately from health probes.
func capabilityHandler(a *agent) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/capabilities" {
			a.serveHTTP(w, r)
			return
		}
		expected := sha256.Sum256([]byte("Bearer " + a.token))
		supplied := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if subtle.ConstantTimeCompare(expected[:], supplied[:]) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(struct {
			Version       int  `json:"version"`
			ConfigSave    bool `json:"config_save"`
			ConfigApply   bool `json:"config_apply"`
			ConfigDisable bool `json:"config_disable"`
			ConfigVersion bool `json:"config_version"`
			RuntimeHealth bool `json:"runtime_health"`
			}{1, true, true, true, true, true})
	})
}
