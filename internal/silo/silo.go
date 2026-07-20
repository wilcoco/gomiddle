// Package silo reads silo weights from the Modbus TCP PLC and keeps the
// latest snapshot in memory, so HTTP handlers never talk to the PLC directly.
package silo

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/goburrow/modbus"

	"github.com/wilcoco/gomiddle/internal/config"
)

// Reading is the decoded weight of one silo.
type Reading struct {
	Silo int     `json:"silo"` // 1-based silo number
	Raw  int16   `json:"raw"`  // signed 16-bit register value
	Tons float64 `json:"tons"` // raw / scale
}

// Snapshot is the most recent poll result served to API clients.
type Snapshot struct {
	Readings  []Reading `json:"readings"`
	UpdatedAt time.Time `json:"updated_at"`
	Error     string    `json:"error,omitempty"` // non-empty if the last poll failed
}

// Poller owns the Modbus connection and polls the PLC on a fixed interval.
type Poller struct {
	cfg config.Config
	log *slog.Logger

	mu   sync.RWMutex
	snap Snapshot
}

func NewPoller(cfg config.Config, log *slog.Logger) *Poller {
	return &Poller{cfg: cfg, log: log}
}

// Snapshot returns the latest poll result. Safe for concurrent use.
func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

// Run polls until ctx is cancelled. It reconnects on every cycle failure,
// which keeps the loop simple and robust against PLC restarts.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	p.poll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *Poller) poll() {
	readings, err := p.read()

	p.mu.Lock()
	defer p.mu.Unlock()
	p.snap = Snapshot{Readings: readings, UpdatedAt: time.Now()}
	if err != nil {
		p.snap.Error = err.Error()
		p.log.Error("silo poll failed", "err", err)
	}
}

func (p *Poller) read() ([]Reading, error) {
	if p.cfg.MockPLC {
		return p.mockReadings(), nil
	}

	handler := modbus.NewTCPClientHandler(p.cfg.SiloPLCAddr)
	handler.Timeout = p.cfg.PLCTimeout
	handler.SlaveId = p.cfg.SiloUnitID
	if err := handler.Connect(); err != nil {
		return nil, err
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	// FC3 Read Holding Registers — the only function code this PLC supports.
	data, err := client.ReadHoldingRegisters(p.cfg.SiloRegisterAddr, p.cfg.SiloCount)
	if err != nil {
		return nil, err
	}

	readings := make([]Reading, 0, p.cfg.SiloCount)
	for i := 0; i < int(p.cfg.SiloCount); i++ {
		// Each register is one big-endian 16-bit word; reinterpret as
		// signed because negative weights are possible.
		raw := int16(uint16(data[2*i])<<8 | uint16(data[2*i+1]))
		readings = append(readings, Reading{
			Silo: i + 1,
			Raw:  raw,
			Tons: float64(raw) / p.cfg.SiloScale,
		})
	}
	return readings, nil
}

// mockReadings produces slowly varying fake weights for local development.
func (p *Poller) mockReadings() []Reading {
	readings := make([]Reading, 0, p.cfg.SiloCount)
	t := float64(time.Now().Unix())
	for i := 0; i < int(p.cfg.SiloCount); i++ {
		tons := 10 + 5*math.Sin(t/60+float64(i))
		raw := int16(tons * p.cfg.SiloScale)
		readings = append(readings, Reading{
			Silo: i + 1,
			Raw:  raw,
			Tons: float64(raw) / p.cfg.SiloScale,
		})
	}
	return readings
}
