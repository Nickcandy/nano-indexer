package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server  ServerConfig
	Mongo   MongoConfig
	Eth     EthConfig
	Scanner ScannerConfig
}

type ServerConfig struct {
	Port string
}

func (c ServerConfig) Addr() string {
	return ":" + c.Port
}

type MongoConfig struct {
	URI      string
	Database string
}

type EthConfig struct {
	RPCURL            string
	ChainID           int64
	Confirmations     uint64
	DefaultStartBlock uint64
	BatchSize         uint64
	PollInterval      time.Duration
}

type ScannerConfig struct {
	Enabled        bool
	TokenAddresses []string
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Port: env("SERVER_PORT", "8080"),
		},
		Mongo: MongoConfig{
			URI:      env("MONGO_URI", "mongodb://localhost:27017"),
			Database: env("MONGO_DATABASE", "nano_indexer"),
		},
		Eth: EthConfig{
			RPCURL:            env("RPC_URL", ""),
			ChainID:           envInt64("CHAIN_ID", 1),
			Confirmations:     envUint64("CONFIRMATIONS", 12),
			DefaultStartBlock: envUint64("DEFAULT_START_BLOCK", 1),
			BatchSize:         envUint64("BATCH_SIZE", 1000),
			PollInterval:      envDuration("POLL_INTERVAL", 10*time.Second),
		},
		Scanner: ScannerConfig{
			Enabled:        envBool("SCANNER_ENABLED", false),
			TokenAddresses: envList("TOKEN_ADDRESSES"),
		},
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key != "" && os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func envInt64(key string, fallback int64) int64 {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envUint64(key string, fallback uint64) uint64 {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(env(key, ""))
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes"
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := env(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		return parsed
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func envList(key string) []string {
	value := env(key, "")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.ToLower(strings.TrimSpace(part))
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}
