package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestStandaloneAgentDiscoveryEndToEnd(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "agent")
	if out, err := exec.Command("go", "build", "-o", binary, "../../../cmd/nodehub-agent").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	token := strings.Repeat("a", 32)
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := listener.Addr().String()
			listener.Close()
			args := []string{"-listen", addr, "-token-file", tokenPath, "-identity-file", filepath.Join(dir, scheme+"-identity"), "-config-file", filepath.Join(dir, scheme+"-config"), "-xray", "/bin/true"}
			if scheme == "http" {
				args = append(args, "-allow-http")
			} else {
				args = append(args, "-tls-cert", certPath, "-tls-key", keyPath)
			}
			cmd := exec.Command(binary, args...)
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			host, port, _ := net.SplitHostPort(addr)
			p, _ := strconv.Atoi(port)
			n := &model.Node{NodeType: model.NodeTypeAgent, Scheme: scheme, Address: host, Port: p, ApiToken: token, AllowPrivateAddress: true}
			if scheme == "https" {
				pin := sha256.Sum256(der)
				n.TlsVerifyMode = "pin"
				n.PinnedCertSha256 = base64.StdEncoding.EncodeToString(pin[:])
			}
			var patch HeartbeatPatch
			for i := 0; i < 100; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				patch, err = (&NodeService{}).Probe(ctx, n)
				cancel()
				if err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err != nil {
				t.Fatal(err)
			}
			caps := patch.ToUI(true).AgentCapabilities
			if caps == nil || caps.Version != 1 || !caps.ConfigSave || !caps.ConfigApply || !caps.RuntimeHealth || patch.XrayState != "stopped" {
				t.Fatalf("capabilities: %+v", caps)
			}
			if _, err := os.Stat(filepath.Join(dir, scheme+"-config")); !os.IsNotExist(err) {
				t.Fatalf("probe unexpectedly created config: %v", err)
			}
			n.ApiToken = "wrong"
			if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
				t.Fatal("wrong token accepted")
			}
			if scheme == "https" {
				n.ApiToken = token
				n.PinnedCertSha256 = base64.StdEncoding.EncodeToString(make([]byte, 32))
				if _, err := (&NodeService{}).Probe(context.Background(), n); err == nil {
					t.Fatal("wrong pin accepted")
				}
			}
		})
	}
}
