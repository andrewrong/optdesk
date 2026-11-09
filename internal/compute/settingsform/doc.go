// Package settingsform issues and checks the token of the owner settings form
// (settingsapi.SettingsForm, built by New), applies the request guard of
// Decision 5 and reads a posted form into the fields that differ. It reads no
// clock (time is an argument), no environment and no file. Design:
// docs/design/settings-form.md#Decisions.
package settingsform
