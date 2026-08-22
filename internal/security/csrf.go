package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

func GenerateCSRFToken(secret, sessionID string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sessionID))
	mac.Write([]byte(fmt.Sprintf("%d", time.Now().Unix()/3600)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ValidateCSRFToken(secret, sessionID, token string) bool {
	expected := GenerateCSRFToken(secret, sessionID)
	return hmac.Equal([]byte(expected), []byte(token))
}
