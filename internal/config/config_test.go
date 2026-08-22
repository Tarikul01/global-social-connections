package config_test

import (
	"os"
	"testing"

	"github.com/findneighbour/findneighbour/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	os.Setenv("DATABASE_URL", "")
	os.Setenv("APP_SECRET", "this-is-a-test-secret-key-32chars!")
	_, err := config.Load()
	require.Error(t, err)
}

func TestLoadSuccess(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgresql://localhost/test")
	os.Setenv("APP_SECRET", "this-is-a-test-secret-key-32chars!")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.AppPort)
}
