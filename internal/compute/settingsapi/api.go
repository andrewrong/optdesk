// Package settingsapi declares the rule parameter registry and the settings
// form of the owner settings page. It holds declarations only; the
// implementations are passed in by cmd/ot. Design:
// docs/design/settings-form.md#Interface.
package settingsapi

import "time"

// RuleParamSpec describes one owner rule parameter. All fields are text; Kind
// is "int" or "decimal".
type RuleParamSpec struct{ Key, Label, Unit, Kind, Default, Min, Max string }

// RuleParams is the registry of the owner rule parameters.
type RuleParams interface {
	// Specs returns the twelve keys in a fixed order.
	Specs() []RuleParamSpec
	// Normalize returns the canonical text of a value; "" is valid and means
	// the default.
	Normalize(key, text string) (string, error)
	// Resolve gives every key: the stored text, else the default.
	Resolve(stored map[string]string) map[string]string
}

// NewRuleParamsFunc builds the registry.
type NewRuleParamsFunc func() RuleParams

// FormToken is the expiry and MAC a posted form carries.
type FormToken struct{ Expires, MAC string }

// FormRequest is what the guard of a POST sees of the request.
type FormRequest struct {
	RemoteAddr, Host, Origin, Listen, ContentType string
	BodySize                                      int64
	Token                                         FormToken
}

// FormField is one parameter text of a posted form.
type FormField struct{ Key, Text string }

// FormState is the settings in force when a form is parsed.
type FormState struct {
	Params     map[string]string
	Exclusions map[int64]bool
}

// FormChange is what a posted form changes: only the differing fields.
// Confirm is the `confirm=1` field.
type FormChange struct {
	Params     []FormField
	Exclusions map[int64]bool
	Confirm    bool
}

// SettingsForm issues the form token and reads a posted form.
type SettingsForm interface {
	// Token returns the token valid at now.
	Token(now time.Time) FormToken
	// Check applies the request guard and the token; the error text is fixed.
	Check(r FormRequest, now time.Time) error
	// Parse returns only the fields that differ from cur; the first refused
	// key is named in the error.
	Parse(values map[string][]string, shown []int64, cur FormState) (FormChange, error)
}

// UploadChecker guards a multipart upload under a cap the caller names. It is
// a separate interface so no implementation of SettingsForm changes; a caller
// reaches it by a type assertion on the form. Design:
// docs/design/flex-xml-import.md#2 upload-guard.
type UploadChecker interface {
	// CheckUpload applies the guard of Check (remote address, Host, Origin,
	// token, expiry) with the content type multipart/form-data and a body size
	// from 0 to maxBody; the error text is fixed.
	CheckUpload(r FormRequest, maxBody int64, now time.Time) error
}

// NewSettingsFormFunc builds a form for a MAC key, a token lifetime and a registry.
type NewSettingsFormFunc func(key []byte, ttl time.Duration, p RuleParams) SettingsForm
