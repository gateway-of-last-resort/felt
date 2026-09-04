package bank

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gateway-of-last-resort/felt/internal/engine"
)

const me = engine.LocalPlayer

func tempLedger(t *testing.T) *JSONLedger {
	t.Helper()
	l, err := OpenJSON(filepath.Join(t.TempDir(), "save.json"))
	if err != nil {
		t.Fatalf("OpenJSON: %v", err)
	}
	return l
}

// A debit larger than the balance must fail and leave the money alone.
func TestDebitBeyondBalance(t *testing.T) {
	l := tempLedger(t)
	start := l.Balance(me)

	if err := l.Debit(me, start+1); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("Debit error = %v, want ErrInsufficientFunds", err)
	}
	if got := l.Balance(me); got != start {
		t.Fatalf("balance = %d after a refused debit, want %d", got, start)
	}

	if err := l.Debit(me, start); err != nil {
		t.Fatalf("Debit: %v", err)
	}
	if !l.Broke() {
		t.Error("Broke() = false at zero balance")
	}
}

func TestNegativeDebitRefused(t *testing.T) {
	l := tempLedger(t)
	start := l.Balance(me)
	if err := l.Debit(me, -10); err == nil {
		t.Fatal("a negative debit was accepted")
	}
	if got := l.Balance(me); got != start {
		t.Errorf("balance = %d, want %d", got, start)
	}
}

// A bailout returns to the starting bankroll, not to whatever is left.
func TestBailout(t *testing.T) {
	l := tempLedger(t)
	if err := l.Debit(me, l.Balance(me)); err != nil {
		t.Fatal(err)
	}
	l.Bailout()
	if got := l.Balance(me); got != DefaultBankroll {
		t.Errorf("balance = %d after a bailout, want %d", got, DefaultBankroll)
	}
}

func TestStatsRecord(t *testing.T) {
	l := tempLedger(t)

	l.Record(me, engine.KindSlots, 25, 0)  // loss
	l.Record(me, engine.KindSlots, 25, 75) // win
	l.Record(me, engine.KindBlackjack, 10, 10)

	s := l.Stats(me)
	slots := s.Get(engine.KindSlots)
	if slots.Rounds != 2 || slots.Wagered != 50 || slots.Won != 75 {
		t.Errorf("slots = %+v, want 2 rounds, 50 wagered, 75 won", slots)
	}
	if slots.Best != 50 {
		t.Errorf("best = %d, want 50", slots.Best)
	}
	if got := s.Net(); got != 25 {
		t.Errorf("Net() = %d, want 25", got)
	}
	if got := s.RTP(engine.KindSlots); got != 1.5 {
		t.Errorf("RTP = %v, want 1.5", got)
	}
	if got := s.RTP(engine.KindRoulette); got != 0 {
		t.Errorf("RTP of an unplayed game = %v, want 0", got)
	}
	if got := s.Rounds(); got != 3 {
		t.Errorf("Rounds() = %d, want 3", got)
	}
}

// The wallet has to survive a restart, which is the whole point of the file.
func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")

	l, err := OpenJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Debit(me, 250); err != nil {
		t.Fatal(err)
	}
	l.Record(me, engine.KindRoulette, 250, 100)
	if err := l.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	again, err := OpenJSON(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got, want := again.Balance(me), DefaultBankroll-250; got != want {
		t.Errorf("balance after reload = %d, want %d", got, want)
	}
	g := again.Stats(me).Get(engine.KindRoulette)
	if g.Rounds != 1 || g.Wagered != 250 || g.Won != 100 {
		t.Errorf("stats after reload = %+v", g)
	}
}

// A missing file is a first run, not an error.
func TestMissingFileIsAFreshWallet(t *testing.T) {
	l, err := OpenJSON(filepath.Join(t.TempDir(), "nothing-here.json"))
	if err != nil {
		t.Fatalf("OpenJSON on a missing file: %v", err)
	}
	if got := l.Balance(me); got != DefaultBankroll {
		t.Errorf("balance = %d, want the default bankroll", got)
	}
}

// A corrupt file must not stop the game starting: defaults come back with the
// error, and the caller decides what to say.
func TestCorruptFileFallsBackToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "save.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := OpenJSON(path)
	if err == nil {
		t.Error("a corrupt wallet was read without complaint")
	}
	if got := l.Balance(me); got != DefaultBankroll {
		t.Errorf("balance = %d after a corrupt read, want the default", got)
	}
}

// The save is written atomically, so an interrupted write cannot leave a
// truncated file where the wallet should be.
func TestSaveLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	l, err := OpenJSON(filepath.Join(dir, "save.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "save.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory holds %v, want just save.json", names)
	}
}
