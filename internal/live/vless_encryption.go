package live

import (
	"crypto/mlkem"
	"encoding/base64"
	"errors"
	"strings"
)

const vlessEncryptionPrefix = "mlkem768x25519plus.native."

func (s *Spec) EnableVLESSEncryption() error {
	key, err := mlkem.GenerateKey768()
	if err != nil {
		return err
	}
	s.Decryption = vlessEncryptionPrefix + "600s." + base64.RawURLEncoding.EncodeToString(key.Bytes())
	s.Encryption = vlessEncryptionPrefix + "0rtt." + base64.RawURLEncoding.EncodeToString(key.EncapsulationKey().Bytes())
	return nil
}

func (s Spec) ClientEncryption() string {
	if s.Encryption == "" {
		return "none"
	}
	return s.Encryption
}

func (s Spec) validateVLESSEncryption(client bool) error {
	if s.Encryption == "" && s.Decryption == "" {
		return nil
	}
	if s.Engine != "xray" || s.Kind != "vless" || s.Transport != "xhttp" {
		return errors.New("此面板的 VLESS Encryption 仅支持 Xray VLESS + XHTTP")
	}
	public, ok := strings.CutPrefix(s.Encryption, vlessEncryptionPrefix+"0rtt.")
	raw, err := base64.RawURLEncoding.DecodeString(public)
	if !ok || err != nil {
		return errors.New("VLESS Encryption 客户端参数无效")
	}
	if _, err := mlkem.NewEncapsulationKey768(raw); err != nil {
		return errors.New("VLESS Encryption 客户端参数无效")
	}
	if client && s.Decryption == "" {
		return nil
	}
	seed, ok := strings.CutPrefix(s.Decryption, vlessEncryptionPrefix+"600s.")
	decoded, err := base64.RawURLEncoding.DecodeString(seed)
	if !ok || err != nil {
		return errors.New("VLESS Encryption 服务端参数无效")
	}
	key, err := mlkem.NewDecapsulationKey768(decoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(key.EncapsulationKey().Bytes()) != public {
		return errors.New("VLESS Encryption 参数不匹配")
	}
	return nil
}
