package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig(t *testing.T) {
	conf := GetConfig()
	assert.NotNil(t, conf)
}
