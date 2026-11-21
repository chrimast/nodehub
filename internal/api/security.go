package api

import (
	"fmt"
	"net/http"
	"vps-control/internal/live"
	"vps-control/internal/store"
)

func applySecurity(s *live.Spec, b liveInput) error {
	s.ProtocolOptions = b.ProtocolOptions
	s.Flow = b.Flow
	if b.VLESSEncryption != nil {
		if !*b.VLESSEncryption {
			s.Encryption, s.Decryption = "", ""
		} else if s.Encryption == "" {
			if err := s.EnableVLESSEncryption(); err != nil {
				return err
			}
		}
	}
	s.ALPN = b.ALPN
	s.Congestion = b.Congestion
	s.WSHost = b.WSHost
	s.ServiceName = b.ServiceName
	s.ManagedUsers = b.ManagedUsers
	s.Username = b.Username
	if s.IsReality() {
		if b.CertificateID != "" || b.Certificate != "" || b.PrivateKey != "" {
			return bad("REALITY uses its X25519 keypair, not a CA certificate")
		}
		current := s.Reality
		if current == nil {
			current = live.NewReality()
		}
		if b.Reality != nil {
			v := *b.Reality
			if v.PrivateKey != "" || v.PublicKey != "" && v.PublicKey != current.PublicKey {
				return bad("REALITY keypair is server-generated; private key cannot be supplied")
			}
			current.Target = v.Target
			current.Port = v.Port
			if v.ShortID != "" {
				current.ShortID = v.ShortID
			}
			if v.Fingerprint != "" {
				current.Fingerprint = v.Fingerprint
			}
		}
		s.Reality = current
		s.Certificate = ""
		s.PrivateKey = ""
	} else {
		s.Reality = nil
	}
	return nil
}

func encryptionCapable(version string) bool {
	var major, minor, patch int
	n, _ := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	return n == 3 && (major > 0 || minor > 25 || minor == 25 && patch >= 7)
}

func requireVLESSAgent(r *http.Request, tx store.Tx, host string, spec live.Spec) error {
	if spec.Encryption == "" && (spec.Outbound == nil || spec.Outbound.Encryption == "") {
		return nil
	}
	var version string
	if err := tx.QueryRow(r.Context(), `SELECT agent_version FROM hosts WHERE tenant_id=$1 AND id=$2`, tenant, host).Scan(&version); err != nil {
		return err
	}
	if !encryptionCapable(version) {
		return problem{409, "VLESS Encryption 需要 Agent 0.25.7 或以上，请先升级 Agent"}
	}
	return nil
}

func requireTunedChainAgent(s live.Spec, version string) error {
	if s.Outbound != nil && s.Outbound.NeedsTunedClientAgent() && !live.ClientTuningCapable(version) {
		return problem{409, "upgrade chain entry Agent to 0.23.17 for client tuning"}
	}
	return nil
}
