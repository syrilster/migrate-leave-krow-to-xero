package config

import (
	"os"
	"strconv"
)

type envConfig struct {
	LogLevel         string
	ServerPort       int
	Version          string
	BaseUrl          string
	XeroKey          string
	XeroSecret       string
	XeroEndpoint     string
	XeroAuthEndpoint string
}

func NewEnvironmentConfig() *envConfig {
	return &envConfig{
		LogLevel:         getEnvString("LOG_LEVEL", "INFO"),
		ServerPort:       getEnvInt("SERVER_PORT", 0),
		Version:          getEnvString("VERSION", ""),
		BaseUrl:          "",
		XeroKey:          getEnvString("XERO_KEY", ""),
		XeroSecret:       getEnvString("XERO_SECRET", ""),
		XeroEndpoint:     getEnvString("XERO_ENDPOINT", ""),
		XeroAuthEndpoint: getEnvString("XERO_AUTH_ENDPOINT", ""),
	}
}

// helper function to read an environment or return a default value
func getEnvString(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

// helper function to read an environment or return a default value
func getEnvInt(key string, defaultVal int) int {
	val, err := strconv.Atoi(getEnvString(key, ""))
	if err == nil {
		return val
	}

	return defaultVal
}
