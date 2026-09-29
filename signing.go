package ogmake

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"unicode/utf16"
)

// Same algorithm as packages/core/src/signing.ts.

func rfc3986Encode(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// utf16Less orders by UTF-16 code unit (JS default string sort), not UTF-8 bytes.
func utf16Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// CanonicalQuery drops sig/debug, sorts keys by UTF-16 code unit and
// RFC3986-encodes each pair.
func CanonicalQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sig" || k == "debug" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, rfc3986Encode(k)+"="+rfc3986Encode(params[k]))
	}
	return strings.Join(parts, "&")
}

// SignQuery returns the base64url (unpadded) HMAC-SHA256 of canonical.
func SignQuery(secret []byte, canonical string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// DecodeSecret decodes a key's base64 signing secret.
func DecodeSecret(secretBase64 string) ([]byte, error) {
	secret, err := base64.StdEncoding.DecodeString(secretBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid_signing_secret")
	}
	return secret, nil
}
