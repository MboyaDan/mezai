package contributors

import "testing"

func TestCanTransition(t *testing.T) {
	all := []string{StatusApplied, StatusQualifying, StatusActive, StatusPaused, StatusRemoved}
	allowed := map[[2]string]map[Via]bool{
		{StatusApplied, StatusQualifying}: {ViaProfile: true},
		{StatusQualifying, StatusActive}:  {ViaApproval: true},
		{StatusActive, StatusPaused}:      {ViaOps: true, ViaTrust: true},
		{StatusPaused, StatusActive}:      {ViaOps: true},
		{StatusApplied, StatusRemoved}:    {ViaOps: true},
		{StatusQualifying, StatusRemoved}: {ViaOps: true},
		{StatusActive, StatusRemoved}:     {ViaOps: true},
		{StatusPaused, StatusRemoved}:     {ViaOps: true},
	}
	for _, from := range all {
		for _, to := range all {
			for _, via := range []Via{ViaProfile, ViaApproval, ViaOps, ViaTrust} {
				want := allowed[[2]string{from, to}][via]
				if got := CanTransition(from, to, via); got != want {
					t.Errorf("%s → %s via %d: got %v, want %v", from, to, via, got, want)
				}
			}
		}
	}
	if CanTransition("unknown", StatusRemoved, ViaOps) {
		t.Error("unknown status removed")
	}
}
