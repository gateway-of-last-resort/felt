# vhs demo.tape  —  records demo.gif for the README.
# Run `make tape` (it builds bin/felt first).

Output demo.gif

Set Shell "bash"
Set FontSize 16
Set Width 1400
Set Height 800
Set Padding 20
Set Theme "Catppuccin Mocha"

Hide
Type "./bin/felt --reset --bankroll 1000"
Enter
Sleep 2s
Show

# The menu.
Sleep 1500ms
Down
Sleep 700ms
Down
Sleep 700ms
Up@300ms 2

# Into the slots.
Enter
Sleep 1s

# Raise the line bet, then spin a few times.
Right
Sleep 400ms
Right
Sleep 800ms

Space
Sleep 4s
Space
Sleep 4s
Space
Sleep 4s

# The paytable.
Type "p"
Sleep 3s
Escape
Sleep 800ms

# Emoji symbols.
Type "g"
Sleep 2s
Space
Sleep 4s
Type "g"
Sleep 1s

# Back out to the menu and sit down at the blackjack table.
Escape
Sleep 1s
Down
Sleep 700ms
Enter
Sleep 1s

# A quick stake, then play a hand out.
Type "2"
Sleep 800ms
Enter
Sleep 2500ms
Type "h"
Sleep 1500ms
Type "s"
Sleep 3s

# On to the roulette layout: a few chips, then spin.
Escape
Sleep 1s
Down
Sleep 700ms
Enter
Sleep 1500ms

Space
Sleep 600ms
Right@300ms 3
Space
Sleep 600ms
Down@300ms 2
Space
Sleep 800ms
Enter
Sleep 5s

# And look at the damage.
Escape
Sleep 1s
Type "S"
Sleep 3s
