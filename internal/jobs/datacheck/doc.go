// Package datacheck is the data_check job: once per XNYS session day, two hours after the close, it runs a fixed
// list of rules over the last CheckWindowSessions session days of each market, stores one run with its findings
// through finding.Writer and sends one message when a finding is new (docs/design/data-check.md, entry 6). It
// only reports: it repairs, deletes and overwrites nothing. It imports the declaration packages of the store and
// no implementation.
package datacheck
