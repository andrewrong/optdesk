// Package strategy declares the one interface an option strategy implements: an id, a rule
// version, parameters with a hash, a pure Evaluate, a pure Rank and a pure Manage. It imports
// contracts and compute only; no store, job or page (docs/design/strategy-framework.md).
package strategy
