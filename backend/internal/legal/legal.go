// Package legal holds the current version of each legal document a user can
// accept. The texts live in frontend/content/legal.ts; a frontend test checks
// that the versions there match these.
package legal

// Document keys as stored in consents.document.
const (
	PrivacyNotice    = "privacy_notice"
	TermsOfUse       = "terms_of_use"
	ContributorTerms = "contributor_terms"
	AcceptableUse    = "acceptable_use"
)

// Current maps each document to the version in force. Raise a version here
// and in legal.ts together; contributors are then asked to accept again.
// 0.2 (2026-11-08): the privacy notice adds the National ID fingerprint and
// the contributor terms add one account per person (D-078).
var Current = map[string]string{
	PrivacyNotice:    "0.2",
	TermsOfUse:       "0.1",
	ContributorTerms: "0.2",
	AcceptableUse:    "0.1",
}

// ContributorRequired are the documents a contributor accepts at sign-up and
// again whenever their version changes (brief 6.2).
var ContributorRequired = []string{PrivacyNotice, ContributorTerms}

// Outstanding returns the required documents whose current version is not in
// accepted (document → latest accepted version), in ContributorRequired order.
func Outstanding(accepted map[string]string) []string {
	out := []string{}
	for _, d := range ContributorRequired {
		if accepted[d] != Current[d] {
			out = append(out, d)
		}
	}
	return out
}
