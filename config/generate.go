package config

import (
	_ "go.uber.org/mock/gomock"
)

//go:generate mockgen -package mocks -destination mocks/mock_config_unmarshaler.go github.com/kasuboski/mediaz/config ConfigUnmarshaler
