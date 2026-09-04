package bank

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gateway-of-last-resort/felt/internal/engine"
)

// Version is the save format. Bumping it lets a later build recognise an
// older file instead of misreading it.
const Version = 1

// DefaultBankroll is the balance a new player starts with, and the amount a
// bailout restores.
const DefaultBankroll int64 = 1000

// save is the on-disk shape of a single-player ledger.
type save struct {
	Version  int   `json:"version"`
	Balance  int64 `json:"balance"`
	Bankroll int64 `json:"bankroll"`
	Stats    Stats `json:"stats"`
}

// JSONLedger is the offline wallet: one player, one file.
//
// It satisfies Ledger and ignores the PlayerID, because locally there is only
// ever one. Writes are explicit — call Save — so a burst of debits during a
// split does not mean a burst of file writes.
type JSONLedger struct {
	mu   sync.Mutex
	path string
	data save
}

// OpenJSON loads the ledger at path. A missing file is not an error: a fresh
// wallet comes back. A corrupt one is reported alongside a fresh wallet, so
// the game can start and say what happened.
func OpenJSON(path string) (*JSONLedger, error) {
	l := &JSONLedger{
		path: path,
		data: save{
			Version:  Version,
			Balance:  DefaultBankroll,
			Bankroll: DefaultBankroll,
			Stats:    NewStats(time.Now()),
		},
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return l, nil
		}
		return l, err
	}

	var s save
	if err := json.Unmarshal(raw, &s); err != nil {
		return l, err
	}
	l.data = normalize(s)
	return l, nil
}

func normalize(s save) save {
	if s.Version == 0 {
		s.Version = Version
	}
	if s.Bankroll <= 0 {
		s.Bankroll = DefaultBankroll
	}
	if s.Balance < 0 {
		s.Balance = 0
	}
	if s.Stats.ByGame == nil {
		s.Stats.ByGame = map[engine.Kind]*GameStats{}
	}
	if s.Stats.Started.IsZero() {
		s.Stats.Started = time.Now()
	}
	return s
}

// Balance satisfies Ledger.
func (l *JSONLedger) Balance(engine.PlayerID) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Balance
}

// Debit satisfies Ledger, leaving the balance untouched on failure.
func (l *JSONLedger) Debit(_ engine.PlayerID, n int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if n < 0 || l.data.Balance < n {
		return ErrInsufficientFunds
	}
	l.data.Balance -= n
	return nil
}

// Credit satisfies Ledger.
func (l *JSONLedger) Credit(_ engine.PlayerID, n int64) error {
	if n <= 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data.Balance += n
	return nil
}

// Record satisfies Ledger.
func (l *JSONLedger) Record(_ engine.PlayerID, game engine.Kind, wagered, won int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data.Stats.Record(game, wagered, won)
}

// Stats satisfies Ledger.
func (l *JSONLedger) Stats(engine.PlayerID) Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Stats
}

// Bankroll is the starting stake a bailout returns to.
func (l *JSONLedger) Bankroll() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Bankroll
}

// SetBankroll changes the starting stake, for the --bankroll flag.
func (l *JSONLedger) SetBankroll(n int64) {
	if n <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data.Bankroll = n
}

// Broke reports whether the player can no longer stake anything.
func (l *JSONLedger) Broke() bool { return l.Balance(engine.LocalPlayer) <= 0 }

// Bailout restores the starting bankroll.
func (l *JSONLedger) Bailout() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.data.Balance = l.data.Bankroll
}

// Save writes the ledger atomically: a temp file beside the real one, then a
// rename, so a crash mid-write cannot truncate the save.
func (l *JSONLedger) Save() error {
	l.mu.Lock()
	l.data.Version = Version
	raw, err := json.MarshalIndent(l.data, "", "  ")
	l.mu.Unlock()
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "save-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }() // a no-op once the rename lands

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	return os.Rename(name, l.path)
}
