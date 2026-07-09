package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	kib        = 1024
	headerJSON = `{"alg":"HS256","typ":"JWT"}`
	demoSecret = "01234567890123456789012345678901"
)

func main() {
	writeFiles := flag.Bool("write", false, "write generated tokens to examples/jwt-1024.txt and examples/jwt-4096.txt")
	showToken := flag.Bool("show-token", false, "print full token content")
	flag.Parse()

	for _, target := range []int{1 * kib, 4 * kib} {
		token, payload, roles, padLen, err := buildToken(target)
		if err != nil {
			panic(err)
		}

		filePath := ""
		if *writeFiles {
			filePath = fmt.Sprintf("examples/jwt-%d.txt", target)
			if err := os.WriteFile(filePath, []byte(token), 0644); err != nil {
				panic(err)
			}
		}

		fmt.Printf("target=%d bytes\n", target)
		fmt.Printf("token=%d bytes\n", len(token))
		fmt.Printf("authorization_header=%d bytes\n", len("Bearer ")+len(token))
		fmt.Printf("payload_json=%d bytes\n", len(payload))
		fmt.Printf("roles=%d\n", len(roles))
		fmt.Printf("pad=%d bytes\n", padLen)
		if filePath != "" {
			fmt.Printf("file=%s\n", filePath)
		}
		if *showToken {
			fmt.Println(token)
		}
		fmt.Println()
	}
}

func buildToken(target int) (string, string, []string, int, error) {
	headerSegmentLen := base64.RawURLEncoding.EncodedLen(len(headerJSON))
	signatureSegmentLen := base64.RawURLEncoding.EncodedLen(sha256.Size)
	payloadSegmentLen := target - headerSegmentLen - signatureSegmentLen - len("..")

	payloadLen, ok := rawLenForEncodedLen(payloadSegmentLen)
	if !ok {
		return "", "", nil, 0, fmt.Errorf("target %d bytes nao gera payload base64url exato", target)
	}

	roles := make([]string, 0)
	for {
		nextRoles := append(append([]string(nil), roles...), fmt.Sprintf("role_%03d", len(roles)+1))
		if minimumPayloadLen(nextRoles) > payloadLen {
			break
		}
		roles = nextRoles
	}

	prefix := payloadPrefix(roles)
	suffix := `"}`
	padLen := payloadLen - len(prefix) - len(suffix)
	if padLen < 0 {
		return "", "", nil, 0, fmt.Errorf("payload minimo ficou maior que %d bytes", payloadLen)
	}

	payload := prefix + strings.Repeat("x", padLen) + suffix
	token := sign(payload)
	if len(token) != target {
		return "", "", nil, 0, fmt.Errorf("token tem %d bytes, esperado %d", len(token), target)
	}

	return token, payload, roles, padLen, nil
}

func rawLenForEncodedLen(encodedLen int) (int, bool) {
	for rawLen := 0; rawLen <= encodedLen; rawLen++ {
		if base64.RawURLEncoding.EncodedLen(rawLen) == encodedLen {
			return rawLen, true
		}
	}
	return 0, false
}

func minimumPayloadLen(roles []string) int {
	return len(payloadPrefix(roles)) + len(`"}`)
}

func payloadPrefix(roles []string) string {
	return fmt.Sprintf(
		`{"sub":"customer_123","email":"dev@example.com","roles":[%s],"iat":0,"exp":3600,"pad":"`,
		rolesJSON(roles),
	)
}

func rolesJSON(roles []string) string {
	var builder strings.Builder
	for i, role := range roles {
		if i > 0 {
			builder.WriteByte(',')
		}
		fmt.Fprintf(&builder, "%q", role)
	}
	return builder.String()
}

func sign(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	signingInput := header + "." + body

	mac := hmac.New(sha256.New, []byte(demoSecret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature
}
