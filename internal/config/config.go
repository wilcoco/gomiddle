// Package config loads all runtime settings from environment variables,
// so secrets and site-specific addresses never live in source code.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// HTTP API
	HTTPAddr string // address the API server listens on, e.g. ":8080"

	// Silo PLC (Modbus TCP)
	SiloPLCAddr      string        // host:port of the Modbus TCP PLC
	SiloUnitID       byte          // Modbus unit (slave) ID
	SiloRegisterAddr uint16        // first holding register address
	SiloCount        uint16        // number of silos (= number of registers)
	SiloScale        float64       // divisor: raw register value / scale = tons
	PollInterval     time.Duration // how often to read the PLC
	PLCTimeout       time.Duration // per-request Modbus timeout

	// Development helper: serve fake silo data instead of contacting a PLC.
	MockPLC bool
}

// Load reads configuration from the environment, applying defaults for
// anything unset. It returns an error for values that fail to parse, so a
// typo in an env var stops the server at startup instead of misbehaving later.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:         getEnv("HTTP_ADDR", ":8080"),
		SiloPLCAddr:      getEnv("SILO_PLC_ADDR", "10.10.41.50:2008"),
		SiloRegisterAddr: 0,
		MockPLC:          getEnv("MOCK_PLC", "false") == "true",
	}

	unitID, err := getEnvInt("SILO_UNIT_ID", 1)
	if err != nil {
		return cfg, err
	}
	cfg.SiloUnitID = byte(unitID)

	count, err := getEnvInt("SILO_COUNT", 6)
	if err != nil {
		return cfg, err
	}
	cfg.SiloCount = uint16(count)

	scale, err := getEnvInt("SILO_SCALE", 100)
	if err != nil {
		return cfg, err
	}
	cfg.SiloScale = float64(scale)

	cfg.PollInterval, err = getEnvDuration("POLL_INTERVAL", 2*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.PLCTimeout, err = getEnvDuration("PLC_TIMEOUT", 3*time.Second)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("env %s: expected integer, got %q", key, v)
	}
	return n, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("env %s: expected duration like \"2s\", got %q", key, v)
	}
	return d, nil
}
