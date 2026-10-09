package semver

import (
	"strings"
	"testing"
)

func TestIsConstraint(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		// plain exact versions
		{"1.24.0", false},
		{"v1.24.0", false},
		{"0.0.0", false},
		{"1.24.0-alpha", false},
		{"1.24.0-alpha.1", false},

		// specials
		{"latest", true},
		{"latest-prerelease", true},
		{"LATEST", true},

		// operators
		{">=1.24.0", true},
		{">1.24.0", true},
		{"<=1.26.0", true},
		{"<1.26.0", true},
		{"=1.24.0", true},
		{"^1.24.0", true},
		{"~1.24.0", true},

		// multi-clause
		{">=1.24.0, <1.26.0", true},
		{">=1.24.0 <1.26.0", true},

		// partial
		{"1.24", true},
		{"1", true},

		// garbage
		{"", false},
		{"not-a-version", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := IsConstraint(tt.in); got != tt.want {
				t.Fatalf("IsConstraint(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseConstraint_Errors(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"^",
		"~",
		">=",
		"<= abc",
		"^abc",
		"~1.2.3.4",
		">=1.2.3.4.5",
		"1.2.3.4",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, err := ParseConstraint(c); err == nil {
				t.Fatalf("expected error for %q", c)
			}
		})
	}
}

func TestConstraint_Match_Exact(t *testing.T) {
	c, err := ParseConstraint("1.24.0")
	if err != nil {
		t.Fatal(err)
	}
	assertMatches(t, c, []string{"1.24.0"}, []string{"1.24.1", "1.23.9", "2.0.0"})
}

func TestConstraint_Match_EqualsOperator(t *testing.T) {
	c, err := ParseConstraint("=1.24.0")
	if err != nil {
		t.Fatal(err)
	}
	assertMatches(t, c, []string{"1.24.0"}, []string{"1.24.1", "1.23.9", "2.0.0"})
}

func TestConstraint_Match_Caret(t *testing.T) {
	t.Run("major >= 1", func(t *testing.T) {
		c, err := ParseConstraint("^1.24.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.24.3", "1.99.99"},
			[]string{"1.23.9", "2.0.0", "0.9.0"})
	})
	t.Run("major == 0", func(t *testing.T) {
		c, err := ParseConstraint("^0.31.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"0.31.0", "0.31.5"},
			[]string{"0.30.9", "0.32.0", "1.0.0"})
	})
	t.Run("partial", func(t *testing.T) {
		c, err := ParseConstraint("^1.24")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.99.99"},
			[]string{"2.0.0", "1.23.9"})
	})
}

func TestConstraint_Match_Tilde(t *testing.T) {
	t.Run("full", func(t *testing.T) {
		c, err := ParseConstraint("~1.24.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.24.9"},
			[]string{"1.25.0", "1.23.9", "2.0.0"})
	})
	t.Run("minor-only", func(t *testing.T) {
		c, err := ParseConstraint("~1.24")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.24.9"},
			[]string{"1.25.0", "1.23.9"})
	})
	t.Run("major-only", func(t *testing.T) {
		c, err := ParseConstraint("~1")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.0.0", "1.99.99"},
			[]string{"2.0.0", "0.99.99"})
	})
}

func TestConstraint_Match_Comparators(t *testing.T) {
	t.Run(">=", func(t *testing.T) {
		c, _ := ParseConstraint(">=1.24.0")
		assertMatches(t, c,
			[]string{"1.24.0", "1.24.3", "2.0.0"},
			[]string{"1.23.9"})
	})
	t.Run(">", func(t *testing.T) {
		c, _ := ParseConstraint(">1.24.0")
		assertMatches(t, c,
			[]string{"1.24.1", "2.0.0"},
			[]string{"1.24.0", "1.23.9"})
	})
	t.Run("<=", func(t *testing.T) {
		c, _ := ParseConstraint("<=1.26.0")
		assertMatches(t, c,
			[]string{"1.26.0", "1.24.0", "0.1.0"},
			[]string{"1.26.1", "2.0.0"})
	})
	t.Run("<", func(t *testing.T) {
		c, _ := ParseConstraint("<1.26.0")
		assertMatches(t, c,
			[]string{"1.25.9", "1.24.0", "0.1.0"},
			[]string{"1.26.0", "1.26.1"})
	})
}

func TestConstraint_Match_AndClauses(t *testing.T) {
	t.Run("comma", func(t *testing.T) {
		c, err := ParseConstraint(">=1.24.0, <1.26.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.25.9"},
			[]string{"1.23.9", "1.26.0", "2.0.0"})
	})
	t.Run("space", func(t *testing.T) {
		c, err := ParseConstraint(">=1.24.0 <1.26.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.25.9"},
			[]string{"1.23.9", "1.26.0"})
	})
	t.Run("space before version is ok", func(t *testing.T) {
		c, err := ParseConstraint(">= 1.24.0, < 1.26.0")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.25.9"},
			[]string{"1.23.9", "1.26.0"})
	})
}

func TestConstraint_Match_LatestSelectors(t *testing.T) {
	t.Run("latest excludes pre-releases", func(t *testing.T) {
		c, err := ParseConstraint("latest")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "0.1.0"},
			[]string{"1.24.0-alpha"})
	})
	t.Run("latest-prerelease includes pre-releases", func(t *testing.T) {
		c, err := ParseConstraint("latest-prerelease")
		if err != nil {
			t.Fatal(err)
		}
		assertMatches(t, c,
			[]string{"1.24.0", "1.24.0-alpha"},
			nil)
	})
}

func TestConstraint_Match_PrereleaseGating(t *testing.T) {
	// Pre-releases are excluded from plain ranges that don't mention them.
	c, _ := ParseConstraint("^1.24.0")
	if c.Match(Parse("1.25.0-alpha")) {
		t.Fatal("plain range should not match pre-release 1.25.0-alpha")
	}
	if !c.Match(Parse("1.25.0")) {
		t.Fatal("plain range should match 1.25.0")
	}

	// But when the constraint explicitly references a pre-release of the same
	// (M,m,p), pre-releases of that tuple become eligible.
	c2, err := ParseConstraint(">=1.24.0-alpha, <1.25.0")
	if err != nil {
		t.Fatal(err)
	}
	if !c2.Match(Parse("1.24.0-alpha")) {
		t.Fatal("expected match for 1.24.0-alpha")
	}
	if !c2.Match(Parse("1.24.0")) {
		t.Fatal("expected match for 1.24.0")
	}
	if c2.Match(Parse("1.24.5-beta")) {
		t.Fatal("1.24.5-beta should not be eligible (M.m.p differs from pre anchor)")
	}
}

func TestHighestMatching(t *testing.T) {
	versions := []string{
		"1.23.5", "1.24.0", "1.24.3", "1.25.0", "1.25.1",
		"2.0.0", "1.26.0-alpha", "1.24.0-alpha",
	}
	t.Run("caret picks newest in range", func(t *testing.T) {
		c, _ := ParseConstraint("^1.24.0")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "1.25.1" {
			t.Fatalf("got (%q, %v), want (1.25.1, true)", got, ok)
		}
	})
	t.Run("tilde picks newest patch", func(t *testing.T) {
		c, _ := ParseConstraint("~1.24.0")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "1.24.3" {
			t.Fatalf("got (%q, %v), want (1.24.3, true)", got, ok)
		}
	})
	t.Run("exact match returns exactly", func(t *testing.T) {
		c, _ := ParseConstraint("1.25.0")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "1.25.0" {
			t.Fatalf("got (%q, %v), want (1.25.0, true)", got, ok)
		}
	})
	t.Run("conjunction", func(t *testing.T) {
		c, _ := ParseConstraint(">=1.24.0, <1.25.0")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "1.24.3" {
			t.Fatalf("got (%q, %v), want (1.24.3, true)", got, ok)
		}
	})
	t.Run("latest ignores pre-release", func(t *testing.T) {
		c, _ := ParseConstraint("latest")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "2.0.0" {
			t.Fatalf("got (%q, %v), want (2.0.0, true)", got, ok)
		}
	})
	t.Run("latest-prerelease picks newest", func(t *testing.T) {
		c, _ := ParseConstraint("latest-prerelease")
		got, ok := HighestMatching(c, versions)
		if !ok || got != "2.0.0" {
			t.Fatalf("got (%q, %v), want (2.0.0, true)", got, ok)
		}
	})
	t.Run("no match returns false", func(t *testing.T) {
		c, _ := ParseConstraint("^5.0.0")
		got, ok := HighestMatching(c, versions)
		if ok {
			t.Fatalf("expected no match, got %q", got)
		}
	})
	t.Run("skips unparseable entries", func(t *testing.T) {
		c, _ := ParseConstraint(">=1.0.0")
		got, ok := HighestMatching(c, []string{"garbage", "not-a-version", "1.2.3"})
		if !ok || got != "1.2.3" {
			t.Fatalf("got (%q, %v), want (1.2.3, true)", got, ok)
		}
	})
}

func TestConstraint_Raw(t *testing.T) {
	c, err := ParseConstraint("  ^1.24.0  ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(c.Raw, "^1.24.0") {
		t.Fatalf("Raw should retain original text, got %q", c.Raw)
	}
}

func assertMatches(t *testing.T, c Constraint, shouldMatch, shouldNotMatch []string) {
	t.Helper()
	for _, s := range shouldMatch {
		if !c.Match(Parse(s)) {
			t.Errorf("expected %s to match %s", s, c.Raw)
		}
	}
	for _, s := range shouldNotMatch {
		if c.Match(Parse(s)) {
			t.Errorf("expected %s NOT to match %s", s, c.Raw)
		}
	}
}
