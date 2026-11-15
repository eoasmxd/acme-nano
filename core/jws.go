package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// jwsClient manages ACME RFC 8555 JWS request signing and Replay-Nonce rotation.
// jwsClient 封装 ACME RFC 8555 的 JWS 签名请求与 Replay-Nonce 状态管理。
type jwsClient struct {
	httpClient *http.Client
	privateKey *ecdsa.PrivateKey
	kid        string
	nonce      string
	nonceURL   string
}

// loadOrCreateECKey loads an existing P-256 ECDSA key from disk or generates a new one.
// loadOrCreateECKey 加载或自动生成本地 P-256 ECDSA 私钥文件。
func loadOrCreateECKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("invalid PEM file: %s", path)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

// jwkMap builds a canonical P-256 JWK representation per RFC 7638.
// jwkMap 构造符合 RFC 7638 字典序规范的 P-256 公钥 JWK 映射。
func jwkMap(pub *ecdsa.PublicKey) map[string]string {
	var xBytes, yBytes [32]byte
	pub.X.FillBytes(xBytes[:])
	pub.Y.FillBytes(yBytes[:])
	return map[string]string{
		"crv": "P-256",
		"kty": "EC",
		"x":   base64.RawURLEncoding.EncodeToString(xBytes[:]),
		"y":   base64.RawURLEncoding.EncodeToString(yBytes[:]),
	}
}

// dns01ChallengeValue computes Base64URL(SHA256(token + "." + thumbprint)) for DNS-01 TXT records.
// dns01ChallengeValue 计算 DNS-01 挑战所需的 Base64URL(SHA256(token + "." + thumbprint)) 值。
func dns01ChallengeValue(token string, pub *ecdsa.PublicKey) (string, error) {
	jwkJSON, err := json.Marshal(jwkMap(pub))
	if err != nil {
		return "", err
	}
	thumb := sha256.Sum256(jwkJSON)
	keyAuth := token + "." + base64.RawURLEncoding.EncodeToString(thumb[:])
	digest := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(digest[:]), nil
}

// buildExternalAccountBinding constructs the HS256 JWS externalAccountBinding object per RFC 8555.
// buildExternalAccountBinding 构造符合 RFC 8555 的 HS256 签名 externalAccountBinding 对象。
func buildExternalAccountBinding(kid, hmacKeyB64, newAccountURL string, pub *ecdsa.PublicKey) (map[string]string, error) {
	rawKey := strings.TrimRight(hmacKeyB64, "=")
	rawKey = strings.NewReplacer("+", "-", "/", "_").Replace(rawKey)
	keyBytes, err := base64.RawURLEncoding.DecodeString(rawKey)
	if err != nil {
		return nil, fmt.Errorf("invalid EAB HMAC key: %w", err)
	}

	protectedJSON, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"kid": kid,
		"url": newAccountURL,
	})
	if err != nil {
		return nil, err
	}
	payloadJSON, err := json.Marshal(jwkMap(pub))
	if err != nil {
		return nil, err
	}

	protectedB64 := base64.RawURLEncoding.EncodeToString(protectedJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(protectedB64 + "." + payloadB64))
	signatureB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return map[string]string{
		"protected": protectedB64,
		"payload":   payloadB64,
		"signature": signatureB64,
	}, nil
}

// fetchNonce requests a fresh Replay-Nonce from the ACME newNonce endpoint.
// fetchNonce 向 ACME newNonce 端点发起 HEAD 请求获取新鲜 Replay-Nonce。
func (c *jwsClient) fetchNonce() (string, error) {
	resp, err := c.httpClient.Head(c.nonceURL)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	nonce := resp.Header.Get("Replay-Nonce")
	if nonce == "" {
		return "", fmt.Errorf("missing Replay-Nonce header from %s", c.nonceURL)
	}
	return nonce, nil
}

// postJWS signs the payload with ES256 JWS, sends it to targetURL, and retries once on badNonce.
// postJWS 对请求体进行 ES256 JWS 签名并发送至目标 ACME 端点，遇到 badNonce 时自动重试一次。
func (c *jwsClient) postJWS(targetURL string, payload any) ([]byte, http.Header, error) {
	for attempt := 0; attempt < 2; attempt++ {
		if c.nonce == "" {
			nonce, err := c.fetchNonce()
			if err != nil {
				return nil, nil, err
			}
			c.nonce = nonce
		}

		protected := map[string]any{
			"alg":   "ES256",
			"nonce": c.nonce,
			"url":   targetURL,
		}
		if c.kid != "" {
			protected["kid"] = c.kid
		} else {
			protected["jwk"] = jwkMap(&c.privateKey.PublicKey)
		}
		c.nonce = ""

		protectedJSON, err := json.Marshal(protected)
		if err != nil {
			return nil, nil, err
		}
		protectedB64 := base64.RawURLEncoding.EncodeToString(protectedJSON)

		var payloadB64 string
		if payload != nil {
			payloadJSON, err := json.Marshal(payload)
			if err != nil {
				return nil, nil, err
			}
			payloadB64 = base64.RawURLEncoding.EncodeToString(payloadJSON)
		}

		signingInput := protectedB64 + "." + payloadB64
		digest := sha256.Sum256([]byte(signingInput))
		r, s, err := ecdsa.Sign(rand.Reader, c.privateKey, digest[:])
		if err != nil {
			return nil, nil, err
		}
		var sig [64]byte
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])

		bodyJSON, err := json.Marshal(map[string]string{
			"protected": protectedB64,
			"payload":   payloadB64,
			"signature": base64.RawURLEncoding.EncodeToString(sig[:]),
		})
		if err != nil {
			return nil, nil, err
		}

		req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyJSON))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Content-Type", "application/jose+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		if nextNonce := resp.Header.Get("Replay-Nonce"); nextNonce != "" {
			c.nonce = nextNonce
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, nil, err
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, resp.Header, nil
		}
		if attempt == 0 && strings.Contains(string(respBody), "badNonce") {
			continue
		}
		return nil, nil, fmt.Errorf("ACME request failed [%d %s]: %s", resp.StatusCode, targetURL, string(respBody))
	}
	return nil, nil, fmt.Errorf("ACME Replay-Nonce retry limit exceeded")
}
