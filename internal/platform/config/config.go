package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerAddr          string
	DatabaseURL         string
	MigrationsDir       string
	APIKeyHeader        string
	APIKeySecret        string
	ShutdownTimeout     time.Duration
	MCPServerName       string
	MCPServerVersion    string
	MCPTransport        string
	MCPLogLevel         string
	MCPEnableWriteTools bool
	MCPMaxPageSize      int
	MCPDefaultRunsLimit int
	MCPEnablePrompts    bool
}

func Load() (Config, error) {
	cfg := Config{
		ServerAddr:          getEnv("SERVER_ADDR", ":8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		MigrationsDir:       getEnv("MIGRATIONS_DIR", "./migrations"),
		APIKeyHeader:        getEnv("API_KEY_HEADER", "X-API-Key"),
		APIKeySecret:        os.Getenv("API_KEY_SECRET"),
		ShutdownTimeout:     getEnvDuration("SHUTDOWN_TIMEOUT_SECONDS", 10),
		MCPServerName:       getEnv("MCP_SERVER_NAME", "opencoverage"),
		MCPServerVersion:    getEnv("MCP_SERVER_VERSION", "dev"),
		MCPTransport:        getEnv("MCP_TRANSPORT", "stdio"),
		MCPLogLevel:         getEnv("MCP_LOG_LEVEL", "info"),
		MCPEnableWriteTools: getEnvBool("MCP_ENABLE_WRITE_TOOLS", false),
		MCPMaxPageSize:      getEnvInt("MCP_MAX_PAGE_SIZE", 100),
		MCPDefaultRunsLimit: getEnvInt("MCP_DEFAULT_RUNS_LIMIT", 20),
		MCPEnablePrompts:    getEnvBool("MCP_ENABLE_PROMPTS", true),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvDuration(key string, defaultSeconds int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(defaultSeconds) * time.Second
	}
	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		return time.Duration(defaultSeconds) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(v)
	if err != nil || value <= 0 {
		return defaultValue
	}
	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func (c Config) Validate() error {
	if c.ServerAddr == "" {
		return fmt.Errorf("server address cannot be empty")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("database url cannot be empty")
	}
	if c.MigrationsDir == "" {
		return fmt.Errorf("migrations dir cannot be empty")
	}
	if c.APIKeySecret == "" {
		return fmt.Errorf("api key secret cannot be empty")
	}
	return nil
}

func (c Config) ValidateMCP() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("database url cannot be empty")
	}
	if c.MigrationsDir == "" {
		return fmt.Errorf("migrations dir cannot be empty")
	}
	if c.MCPServerName == "" {
		return fmt.Errorf("mcp server name cannot be empty")
	}
	if c.MCPServerVersion == "" {
		return fmt.Errorf("mcp server version cannot be empty")
	}
	if c.MCPTransport != "stdio" {
		return fmt.Errorf("unsupported mcp transport %q", c.MCPTransport)
	}
	if c.MCPLogLevel != "debug" && c.MCPLogLevel != "info" && c.MCPLogLevel != "warn" && c.MCPLogLevel != "error" {
		return fmt.Errorf("unsupported mcp log level %q", c.MCPLogLevel)
	}
	if c.MCPMaxPageSize <= 0 {
		return fmt.Errorf("mcp max page size must be positive")
	}
	if c.MCPDefaultRunsLimit <= 0 {
		return fmt.Errorf("mcp default runs limit must be positive")
	}
	if c.MCPEnableWriteTools && c.MCPTransport == "stdio" {
		return fmt.Errorf("mcp write tools require a header-capable transport")
	}
	if c.MCPEnableWriteTools && c.APIKeySecret == "" {
		return fmt.Errorf("api key secret cannot be empty when mcp write tools are enabled")
	}
	return nil
}
