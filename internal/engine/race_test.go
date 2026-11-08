//go:build race

package engine

// raceEnabled is true under -race, which slows code several times over, so
// timing budgets do not apply.
const raceEnabled = true
