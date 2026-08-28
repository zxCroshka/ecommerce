package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zxCroshka/ecommerce/services/gateway/internal/config"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  config.LoggingConfig
		wantErr bool
	}{
		{name: "json", config: config.LoggingConfig{Level: "info", Format: "json"}},
		{name: "text", config: config.LoggingConfig{Level: "debug", Format: "text"}},
		{name: "invalid level", config: config.LoggingConfig{Level: "verbose", Format: "json"}, wantErr: true},
		{name: "invalid format", config: config.LoggingConfig{Level: "info", Format: "xml"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := newLogger(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, logger)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, logger)
		})
	}
}
