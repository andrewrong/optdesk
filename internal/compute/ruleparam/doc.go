// Package ruleparam holds the registry of the twelve owner rule parameters
// (settingsapi.RuleParams, built by New): kind, default, bounds, label and
// unit as one table, and the exact decimal text of a value made with
// math/big. It does no I/O and reads no clock. Design:
// docs/design/settings-form.md#Decisions.
package ruleparam
