package httpapi_test

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/MboyaDan/mezai/backend/internal/httpapi"
)

// Every /v1 route must be in openapi.yaml (CLAUDE.md: the spec is the
// contract and changes in the same PR as the endpoint), and every /v1 path in
// the spec must exist.
func TestOpenAPICoversRoutes(t *testing.T) {
	f, err := os.Open("../../openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pathRe := regexp.MustCompile(`^  (/v1/\S+):\s*$`)
	methodRe := regexp.MustCompile(`^    (get|post|put|patch|delete):\s*$`)
	spec := map[string]bool{}
	current := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if m := pathRe.FindStringSubmatch(line); m != nil {
			current = m[1]
			continue
		}
		if !strings.HasPrefix(line, "    ") && strings.TrimSpace(line) != "" {
			current = ""
		}
		if m := methodRe.FindStringSubmatch(line); m != nil && current != "" {
			spec[strings.ToUpper(m[1])+" "+current] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	_, rules := httpapi.NewRouterWithRules(httpapi.Deps{Auth: &httpapi.AuthDeps{}})
	param := regexp.MustCompile(`:(\w+)`)
	routes := map[string]bool{}
	for _, r := range rules {
		key := r.Method + " " + param.ReplaceAllString(r.Path, "{$1}")
		routes[key] = true
		if !spec[key] {
			t.Errorf("%s is registered but missing from openapi.yaml", key)
		}
	}
	for key := range spec {
		if !routes[key] {
			t.Errorf("%s is in openapi.yaml but not registered", key)
		}
	}
	if len(routes) < 30 {
		t.Fatalf("only %d routes found", len(routes))
	}
}
