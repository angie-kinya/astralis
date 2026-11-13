package registry

import (
	"astralis/gateway/internal/game/sudoku"
	"astralis/gateway/internal/game/words"
)

// the puzzles, in a file of their own so the board game above never waited on them
func init() {
	register(words.New())
	register(sudoku.New())
}
