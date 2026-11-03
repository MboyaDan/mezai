package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolveKind(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	staff := grant{roleKey: "ops_manager", roleScope: "platform", perm: "project.manage"}
	contrib := grant{roleKey: "contributor", roleScope: "platform", perm: "task.submit"}
	clientA := grant{roleKey: "client_admin", roleScope: "org", perm: "org.project.read", orgID: &orgA}
	clientB := grant{roleKey: "client_viewer", roleScope: "org", perm: "org.project.read", orgID: &orgB}

	if k, _, _, ok := resolveKind([]grant{staff}); !ok || k != KindStaff {
		t.Errorf("staff: %v %v", k, ok)
	}
	if k, org, _, ok := resolveKind([]grant{clientA}); !ok || k != KindClient || org != orgA {
		t.Errorf("client: %v %v %v", k, org, ok)
	}
	if k, _, _, ok := resolveKind([]grant{contrib}); !ok || k != KindContributor {
		t.Errorf("contributor: %v %v", k, ok)
	}
	for name, gs := range map[string][]grant{
		"no roles":              nil,
		"staff and client":      {staff, clientA},
		"staff and contributor": {staff, contrib},
		"two organisations":     {clientA, clientB},
	} {
		if _, _, _, ok := resolveKind(gs); ok {
			t.Errorf("%s: should be refused", name)
		}
	}
}

func TestSystemAdminImpliesEverything(t *testing.T) {
	p := &Principal{perms: map[string]bool{PermSystemAdmin: true}}
	if !p.Has("payout.approve") {
		t.Fatal("system.admin must imply every permission")
	}
	q := &Principal{perms: map[string]bool{"review.read": true}}
	if q.Has("review.decide") || !q.Has("review.read") {
		t.Fatal("ordinary permissions are exact")
	}
	var none *Principal
	if none.Has("review.read") {
		t.Fatal("nil principal has nothing")
	}
}
