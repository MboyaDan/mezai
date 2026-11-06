package legal

import (
	"reflect"
	"testing"
)

func TestOutstanding(t *testing.T) {
	if got := Outstanding(nil); !reflect.DeepEqual(got, ContributorRequired) {
		t.Fatalf("nothing accepted: %v", got)
	}
	all := map[string]string{}
	for _, d := range ContributorRequired {
		all[d] = Current[d]
	}
	if got := Outstanding(all); len(got) != 0 {
		t.Fatalf("all current: %v", got)
	}
	all[PrivacyNotice] = "0.0"
	if got := Outstanding(all); !reflect.DeepEqual(got, []string{PrivacyNotice}) {
		t.Fatalf("old privacy version: %v", got)
	}
	for _, d := range ContributorRequired {
		if Current[d] == "" {
			t.Fatalf("%s has no current version", d)
		}
	}
}
