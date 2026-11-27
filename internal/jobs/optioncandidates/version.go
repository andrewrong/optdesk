package optioncandidates

// RuleVersion is the rule_version column of every row this job writes. It changes whenever a rule of the put gates
// or tiers changes what the rows say; TestRuleVersionDigest fails until it is bumped and the digest of the new
// version is added. The data check compares it with the stored value.
const RuleVersion = "put_v2"
