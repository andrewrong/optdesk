// Package cutover holds the jobs of the cut-over from option-trader (docs/design/cut-over.md): the account
// history import and verify, and the figure comparison. The functions it implements are the function types
// jobs.ImportAccountFunc, jobs.VerifyAccountFunc and jobs.CompareFiguresFunc. jsonl.go is the JSON Lines
// reader and the canonical hash they share.
package cutover
