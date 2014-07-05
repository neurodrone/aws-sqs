package sqs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

func GenerateSignature(sqsURI, method, secret string, uv url.Values) string {
	u, err := url.Parse(sqsURI)
	if err != nil {
		return ""
	}

	sigPayload := strings.Join([]string{
		method,
		u.Host,
		u.Path,
		uv.Encode(),
	}, "\n")

	h := hmac.New(sha256.New, []byte(secret))
	fmt.Fprint(h, sigPayload)

	b64 := base64.StdEncoding
	sig := make([]byte, b64.EncodedLen(h.Size()))
	b64.Encode(sig, h.Sum(nil))

	return string(sig)
}
