// Command felt is a casino for the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/gateway-of-last-resort/felt/internal/app"
	"github.com/gateway-of-last-resort/felt/internal/bank"
	"github.com/gateway-of-last-resort/felt/internal/rng"
	"github.com/gateway-of-last-resort/felt/internal/store"
)

func main() {
	bankroll := flag.Int64("bankroll", 0, "starting credits for a fresh wallet")
	server := flag.String("server", "", "address of a felt server, host:port")
	reset := flag.Bool("reset", false, "delete the wallet and settings, and start over")
	debug := flag.Bool("debug", false, "write a debug log to felt.log")
	flag.Parse()

	if err := run(*bankroll, *server, *reset, *debug); err != nil {
		fmt.Fprintln(os.Stderr, "felt:", err)
		os.Exit(1)
	}
}

func run(bankroll int64, server string, reset, debug bool) error {
	if debug {
		// Anything printed to stdout would land in the middle of the table,
		// so debugging goes to a file.
		f, err := tea.LogToFile("felt.log", "felt")
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
	}

	if reset {
		if err := store.Reset(); err != nil {
			return err
		}
	}

	settings, err := store.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "felt: could not read settings, using defaults:", err)
	}
	if server != "" {
		settings.Server = server
	}

	savePath, err := store.SavePath()
	if err != nil {
		return err
	}
	ledger, err := bank.OpenJSON(savePath)
	if err != nil {
		// A broken wallet is not worth refusing to start over: play with a
		// fresh one and say what happened.
		fmt.Fprintln(os.Stderr, "felt: could not read the wallet, starting fresh:", err)
	}
	if bankroll > 0 {
		ledger.SetBankroll(bankroll)
		if reset {
			ledger.Bailout()
		}
	}

	// The alt screen is switched on in the view, not here: in Bubble Tea v2
	// the screen mode is part of what is drawn.
	p := tea.NewProgram(app.New(ledger, settings, rng.New()))

	final, err := p.Run()
	if err != nil {
		return err
	}

	// The wallet is written after every settled round; this catches a quit
	// taken between rounds.
	if m, ok := final.(app.Model); ok {
		if err := m.Persist(); err != nil {
			return fmt.Errorf("saving: %w", err)
		}
	}
	return nil
}
