package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestCapabilityHandlerAuthAndContract(t *testing.T) {
	a := &agent{token: "secret", id: "node"}
	h := capabilityHandler(a)
	for _, tc := range []struct {
		method, path, token string
		want                int
	}{
		{"GET", "/v1/capabilities", "wrong", 401},
		{"POST", "/v1/capabilities", "secret", 404},
		{"GET", "/v1/capabilities", "secret", 200},
		{"GET", "/v1/status", "secret", 200},
	} {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.path == "/v1/capabilities" && tc.want == 200 {
			var got struct {
				Version       int  `json:"version"`
				ConfigSave    bool `json:"config_save"`
				ConfigApply   bool `json:"config_apply"`
				ConfigDisable bool `json:"config_disable"`
				RuntimeHealth bool `json:"runtime_health"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Version != 1 || !got.ConfigSave || !got.ConfigApply || !got.ConfigDisable || !got.RuntimeHealth {
				t.Fatalf("incorrect capabilities: %+v", got)
			}
		}
	}
}
