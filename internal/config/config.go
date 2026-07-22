// Package config loads all runtime settings from environment variables,
// so secrets and site-specific addresses never live in source code.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	SiloChangeTons   float64       // forward a silo value only if it moved this many tons
	PollInterval     time.Duration // how often to read the PLC
	PLCTimeout       time.Duration // per-request Modbus timeout

	// Injection-molding PLCs (Mitsubishi MC Protocol 3E binary)
	InjMachines     []Machine     // one entry per injection machine
	InjPollInterval time.Duration // how often to poll each machine

	// Development helper: serve fake silo data instead of contacting a PLC.
	MockPLC bool
}

// Machine identifies one injection-molding machine.
type Machine struct {
	No   string // machine number, single digit "1"-"9" per the Odoo API spec
	Addr string // host:port of the Mitsubishi PLC
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

	cfg.SiloChangeTons, err = getEnvFloat("SILO_CHANGE_TONS", 0.3)
	if err != nil {
		return cfg, err
	}

	cfg.PollInterval, err = getEnvDuration("POLL_INTERVAL", 2*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.PLCTimeout, err = getEnvDuration("PLC_TIMEOUT", 3*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.InjPollInterval, err = getEnvDuration("INJ_POLL_INTERVAL", 1*time.Second)
	if err != nil {
		return cfg, err
	}

	cfg.InjMachines, err = parseMachines(getEnv("INJ_MACHINES", "1=10.10.41.21:3001,2=10.10.41.31:3001"))
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

// parseMachines parses "1=host:port,2=host:port" into Machine values.
func parseMachines(s string) ([]Machine, error) {
	var machines []Machine
	for _, entry := range strings.Split(s, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		no, addr, ok := strings.Cut(entry, "=")
		if !ok || no == "" || addr == "" {
			return nil, fmt.Errorf("env INJ_MACHINES: bad entry %q, want no=host:port", entry)
		}
		machines = append(machines, Machine{No: no, Addr: addr})
	}
	return machines, nil
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

func getEnvFloat(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("env %s: expected number, got %q", key, v)
	}
	return f, nil
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
