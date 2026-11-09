// Package candidate holds the store of the option candidates: Open connects with the role ot_w_candidate and
// returns a Store that appends rows to candidates.option_candidate (store.CandidateWriter) and reads the newest
// run of a day through the view rd.option_candidate_r1 (store.CandidateReader).
package candidate
