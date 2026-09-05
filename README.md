# felt

A casino for the terminal, built on [Bubble Tea v2](https://charm.land).

Felt is the green cloth on a gaming table. It is also a warm-up project: the point is
the animation timing, the state machines and the layering, not shipping production
software. The credits are imaginary. The maths is not — the slot machine's return and
roulette's house edge are both computed exactly and asserted by tests.

The plan has two halves. This is the first: a complete offline game, built so that the
second — shared tables over SSH — is new files rather than a rewrite.

## Status

| Stage | What | State |
|---|---|---|
| 0 | Layers, wallet, theme, router, the Online launcher | done |
| 1 | Slots — spring-braked reels, five paylines, paytable | done |
| 2 | Blackjack — six decks, splits, insurance, surrender | done |
| 3 | Roulette — the full betting layout, cursor, ball | done |
| 4 | Video poker — Jacks or Better, 9/6 | done |
| 5 | Statistics, help, demo recording | help and stats done |
| 6+ | `feltd`: rooms, shared tables, holdem | not started |

## Installing

Grab the archive for your machine from
[releases](https://github.com/gateway-of-last-resort/felt/releases), unpack it, and run
`felt`. There is nothing to install and no runtime to have: it is one static binary, for
macOS, Linux and Windows, on both Intel and ARM.

`felt --version` reports the build, which is the first thing worth pasting into a bug
report.

## Running from source

```bash
go run ./cmd/felt
```

Flags: `--bankroll N` sets the starting credits, `--server host:port` points the Online
item somewhere else, `--reset` deletes the wallet and settings, `--debug` writes a log to
`felt.log` (stdout would land in the middle of the table).

The layout targets 160×40. Below 100×30 it tightens; below 80×24 it asks you to resize
rather than shearing the table.

## Keys

| Key | Everywhere |
|---|---|
| `↑ ↓ ← →` | move |
| `enter` | select |
| `esc` | back to the menu — refused with money on the table |
| `q` | quit — refused mid-round, and never while typing a bet |
| `S` | statistics |
| `?` | help |
| `ctrl+c` | quit and save, from anywhere |

| Key | Slots | Blackjack | Roulette | Video poker |
|---|---|---|---|---|
| `space` | spin, or skip | — | place a chip, or skip | hold the card |
| `enter` | — | deal, next hand | spin | deal, then draw |
| `← →` | stake per line | stake (`1`–`4`, `b` type) | move | move |
| `↑ ↓` | active lines | — | move | coins |
| `1`–`5` | — | quick stake | — | hold that card |
| `h` `s` | — | hit, stand | — | — |
| `d` `p` `u` | — | double, split, surrender | — | — |
| `i` `n` | — | insurance | — | — |
| `+` `-` | — | — | chip value | coins |
| `m` | — | — | — | max bet and deal |
| `backspace` | — | — | take the chip back | — |
| `r` `c` | — | — | repeat, clear | — |
| `p` `g` | paytable, symbols | — | — | — |

## Architecture

Four layers, strictly downwards:

```
games/      presentation — animation, keys, drawing a Snapshot
   │ Action ↓          ↑ EventsMsg
driver/     Local: engine + wallet in this process   (later: Room, a channel to a server)
   │
engine/     pure state machines — no Bubble Tea, no money, no clock
   │
deck · rng · bank.Ledger
```

Four rules hold it together, and they are the reason the online half will not be a
rewrite:

- **The engine imports nothing.** No `tea`, no wallet, and time only ever as a `now`
  parameter. That is what makes the rules testable without a terminal, and what lets the
  same code run in a server room.
- **The engine never sees a balance.** A bet is a number on a spot. Whoever owns the
  ledger — `driver.Local` here, a room later — debits the stake *before* the action is
  applied, and refunds it if the engine refuses. There are two ways money reaches a
  table: bet by bet from the wallet, as all three games here do, or once at the door as
  a buy-in, which is how holdem will work — the engine then moves chips between stacks
  and pots itself, and hands back what is left when the player stands up.
- **The presentation only knows Snapshots and events.** Whether they came from a
  goroutine next door or a machine across the room is not its business.
- **Results are decided before they are animated.** The engine deals a whole blackjack
  round the instant a bet lands; the screen then plays the cards out a quarter-second
  apart. The reels catch up with a spin that has already happened.

## The slot machine

Three reels of 32 stops, five paylines, wilds substituting. Theoretical return is
**94.9%**, and 12.7% of lines pay — about half of all spins return something with five
lines live. Both come from walking all 32³ combinations rather than simulating, which is
why a test can assert on them exactly:

```bash
make rtp
```

Reels do not stop dead. Each cruises until its stop time, then hands over to a
[harmonica](https://github.com/charmbracelet/harmonica) spring that pulls it onto the
target — so it overshoots by a symbol and settles back. A test asserts the overshoot,
because losing it is the difference between working and feeling right.

## Blackjack

Six decks cut at three quarters, dealer stands on soft 17, blackjack pays 3:2, double
after split, split aces get one card, late surrender, insurance at 2:1. About half a
percent to the house played perfectly — which is why the insurance prompt says out loud
that taking it costs 5.9%.

**Bets are even numbers.** Blackjack pays 3:2 and insurance costs half the stake, so an
odd bet cannot settle in whole credits. Rather than rounding in someone's favour, the
table only takes even stakes, and says so when it rounds one down.

The dealer peeks under a ten or an ace before the player acts. That is what stops someone
doubling into a hand that was already lost.

## Roulette

Single zero, the real wheel order, and the whole betting layout: straights, splits,
streets, corners, six lines, columns, dozens and the even-money row — 152 spots in all.

Every one of them has the same house edge, 1/37, and a test checks all 152 rather than
trusting the payout table. That uniformity is the most surprising thing about roulette
and the easiest thing to break by mistyping one payout.

The layout is a graph, not a grid: half the bets sit on the lines *between* numbers, so
the cursor walks spots rather than cells, and the field is drawn into a character grid so
a chip can land on a border. Spots are generated from a half-grid — even coordinates are
cell centres, odd ones the boundaries — and what a spot covers falls out of which cells
it touches. A test walks the cursor over every spot at every supported size.

## Video poker

Jacks or Better on the 9/6 schedule — a full house pays 9 and a flush 6, per coin. Those
two lines are what a casino shaves to set the return: 9/6 pays 99.5% to perfect play,
8/5 barely 97%. A royal pays 250 a coin, or 800 at five coins, which is the only reason
to bet the maximum.

The evaluator behind it is shared with holdem, so it is tested harder than the game
needs: one test walks all 2,598,960 five-card hands and checks the count of each
category against the known frequencies.

`EV` computes the exact expectation of any hold by enumerating every possible draw —
1,533,939 of them in the worst case, under a second. That makes the strategy testable
rather than merely plausible: a test asserts that a made flush containing four to the
royal should be broken, which is the first decision to flip if a payout is ever mistyped.

## Layout

```
cmd/felt/            client entry point
internal/
  app/               root model — routing, theme, chrome, the Online item
  engine/            pure rules: slots, blackjack, roulette, poker
  driver/            Local — the engine plus the wallet, in this process
  games/             presentation — one model per table
  online/            reachability check, client key, the ssh exec
  bank/              Ledger interface and the JSON wallet
  ui/                theme, cards, chips, toasts, countdown, layout
  menu/  deck/  rng/  store/
```

## Development

```bash
make test      # go test ./... -race
make short     # skips the two exhaustive sweeps
make lint      # golangci-lint
make rtp       # print the machine's exact return and the roulette edge
make snapshot  # build every platform's archive, publishing nothing
make tape      # record demo.gif with vhs
```

Releasing is a tag: `git tag -s v0.1.0 -m "felt v0.1.0" && git push origin v0.1.0`. The
workflow builds the archives and puts them on the releases page, with the version stamped
into each binary at link time.

To look at a screen without launching anything:

```bash
FELT_DUMP=1 go test ./internal/app -run TestDumpFrames -v
```

## Two wallets, later

The offline wallet is a JSON file you can edit in five seconds. A server will keep its
own in bbolt, and the two will never synchronise in either direction. Offline is a
sandbox; online is the real score.

## Licence

[PolyForm Noncommercial 1.0.0](https://polyformproject.org/licenses/noncommercial/1.0.0/).

Play it, read it, take it apart, run a table for your friends, build on it for a hobby
project — all of that is a permitted purpose. Making money from it is not licensed. It is
a casino, and the one thing it should never become is somebody's income.

Copyright remains with the author. The dependencies keep their own licences, which are
permissive and unaffected by this one.
