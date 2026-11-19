// Package yfinance is the adapter of sources.BarSource that runs the one-shot refbars command of
// pyside (S1 of docs/design/base-data-page.md) and reads its JSON answer. It holds no Python and no
// address of the outside service; it is the only place that splits a bar request into batches (S6).
package yfinance
