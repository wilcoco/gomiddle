// Package injection polls one Mitsubishi injection-molding PLC: it mirrors
// the flicker heartbeat (D5000 → D8000) so the PLC knows the middleware is
// alive, and keeps a snapshot of the machine's state for the API.
package injection

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/wilcoco/gomiddle/internal/config"
	"github.com/wilcoco/gomiddle/internal/mcproto"
)

// Register layout constants from the memory map (PLC → middleware block).
const (
	regBase       = 5000 // first register we read: D5000
	regCount      = 41   // through D5040 in one batch read
	idxFlicker    = 0    // D5000: 1-second toggle, PLC liveness
	idxStatus     = 10   // D5010~D5013: equipment status
	idxCodeReq    = 18   // D5018: product-code request
	idxCodeMatch  = 19   // D5019: match result (1=match, 2=mismatch)
	idxRecipe     = 20   // D5020~D5039: recipe product code, packed ASCII
	idxSerialReq  = 40   // D5040: serial issuance request
	regFlickerOut = 8000 // D8000: mirrored heartbeat, middleware → PLC
)

// Snapshot is one machine's latest observed state.
type Snapshot struct {
	Machine    string    `json:"machine"`
	Connected  bool      `json:"connected"`
	Flicker    int       `json:"flicker"`     // raw heartbeat bit as last seen
	Status     [4]int    `json:"status"`      // D5010~D5013
	CodeReq    int       `json:"code_req"`    // D5018
	CodeMatch  int       `json:"code_match"`  // D5019
	RecipeCode string    `json:"recipe_code"` // D5020~D5039
	SerialReq  int       `json:"serial_req"`  // D5040
	UpdatedAt  time.Time `json:"updated_at"`
	Error      string    `json:"error,omitempty"`
}

// Poller polls a single machine. Run one Poller per machine.
type Poller struct {
	machine config.Machine
	cfg     config.Config
	log     *slog.Logger

	client *mcproto.Client // nil when disconnected; recreated on next poll

	mu   sync.RWMutex
	snap Snapshot
}

func NewPoller(machine config.Machine, cfg config.Config, log *slog.Logger) *Poller {
	return &Poller{
		machine: machine,
		cfg:     cfg,
		log:     log.With("machine", machine.No),
		snap:    Snapshot{Machine: machine.No},
	}
}

// Snapshot returns the latest state. Safe for concurrent use.
func (p *Poller) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snap
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.InjPollInterval)
	defer ticker.Stop()
	defer func() {
		if p.client != nil {
			p.client.Close()
		}
	}()

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
	snap, err := p.readOnce()
	snap.Machine = p.machine.No
	snap.UpdatedAt = time.Now()
	if err != nil {
		snap.Error = err.Error()
		p.log.Error("injection poll failed", "err", err)
	}

	p.mu.Lock()
	p.snap = snap
	p.mu.Unlock()
}

func (p *Poller) readOnce() (Snapshot, error) {
	if p.cfg.MockPLC {
		return p.mockSnapshot(), nil
	}

	if p.client == nil {
		c, err := mcproto.Dial(p.machine.Addr, p.cfg.PLCTimeout)
		if err != nil {
			return Snapshot{}, err
		}
		p.client = c
	}

	words, err := p.client.ReadD(regBase, regCount)
	if err != nil {
		// Drop the connection so the next poll redials; this heals PLC
		// restarts and network blips without extra state tracking.
		p.client.Close()
		p.client = nil
		return Snapshot{}, err
	}

	// Mirror the heartbeat back. The PLC watches D8000 follow D5000 to
	// know the middleware is alive (FLICKER_TIMEOUT alarm otherwise).
	if err := p.client.WriteD(regFlickerOut, []uint16{words[idxFlicker]}); err != nil {
		p.client.Close()
		p.client = nil
		return Snapshot{}, err
	}

	snap := Snapshot{
		Connected:  true,
		Flicker:    mcproto.ASCIIBit(words[idxFlicker]),
		CodeReq:    mcproto.ASCIIBit(words[idxCodeReq]),
		CodeMatch:  mcproto.ASCIIDigit(words[idxCodeMatch]), // ASCII '0'/'1'/'2' on real PLC
		RecipeCode: mcproto.DecodeString(words[idxRecipe : idxRecipe+20]),
		SerialReq:  mcproto.ASCIIBit(words[idxSerialReq]),
	}
	for i := 0; i < 4; i++ {
		snap.Status[i] = mcproto.ASCIIBit(words[idxStatus+i])
	}
	return snap, nil
}

func (p *Poller) mockSnapshot() Snapshot {
	return Snapshot{
		Connected:  true,
		Flicker:    int(time.Now().Unix() % 2), // fake 1-second toggle
		Status:     [4]int{1, 0, 0, 0},
		RecipeCode: "PN-MOCK-" + p.machine.No,
	}
}
