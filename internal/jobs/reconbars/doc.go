// Package reconbars is the recon_bars job (pair R6 of docs/design/base-data-page.md, entry 21): it reads our winner
// rows of basis trades of the last 5 sessions for the watch list, asks the second bar source for the same pairs in
// one call, compares them with the injected compare function and stores one run through the recon store writer.
// It imports no implementation package of compute, of the store or of a source.
package reconbars
