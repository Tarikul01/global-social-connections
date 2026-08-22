package security_test

import (
	"testing"

	"github.com/findneighbour/findneighbour/internal/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordHashAndVerify(t *testing.T) {
	params := security.DefaultArgonParams
	hash, err := security.HashPassword("securepassword123", params)
	require.NoError(t, err)
	ok, err := security.VerifyPassword("securepassword123", hash)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = security.VerifyPassword("wrongpassword", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCSRFToken(t *testing.T) {
	secret := "test-secret-key-for-csrf-validation"
	token := security.GenerateCSRFToken(secret, "session-123")
	assert.True(t, security.ValidateCSRFToken(secret, "session-123", token))
	assert.False(t, security.ValidateCSRFToken(secret, "session-456", token))
}
