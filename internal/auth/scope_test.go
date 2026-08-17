package auth

import "testing"

func TestNewScopesDedupAndTrim(t *testing.T) {
	// 混入重复项与首尾空白，验证去重和 trim
	s := NewScopes([]string{"read", " read ", "read", ""})
	if len(s) != 1 {
		t.Fatalf("len(scopes) = %d, want 1", len(s))
	}
	if !s.Has(ScopeRead) {
		t.Errorf("should have read scope")
	}
}

func TestScopesHasAdminWildcard(t *testing.T) {
	s := NewScopes([]string{"admin"})
	if !s.Has(ScopeRead) {
		t.Errorf("admin should imply read")
	}
	if !s.Has(ScopeWrite) {
		t.Errorf("admin should imply write")
	}
}

func TestScopesHasAny(t *testing.T) {
	s := NewScopes([]string{"read"})
	if !s.HasAny(ScopeWrite, ScopeRead) {
		t.Errorf("HasAny should be true when read present")
	}
	if s.HasAny(ScopeWrite, ScopePlugins) {
		t.Errorf("HasAny should be false when none present")
	}
}

func TestScopesStringsSorted(t *testing.T) {
	s := NewScopes([]string{"write", "read"})
	got := s.Strings()
	want := []string{"read", "write"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
