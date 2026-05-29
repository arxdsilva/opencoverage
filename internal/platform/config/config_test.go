package config

import "testing"

func TestValidateMCPRejectsWriteToolsOnStdio(t *testing.T) {
	cfg := Config{
		DatabaseURL:         "postgres://example",
		MigrationsDir:       "./migrations",
		MCPServerName:       "opencoverage",
		MCPServerVersion:    "test",
		MCPTransport:        "stdio",
		MCPLogLevel:         "info",
		MCPEnableWriteTools: true,
		MCPMaxPageSize:      100,
		MCPDefaultRunsLimit: 20,
		APIKeySecret:        "secret",
	}

	if err := cfg.ValidateMCP(); err == nil {
		t.Fatalf("expected write tools on stdio to be rejected")
	}
}
