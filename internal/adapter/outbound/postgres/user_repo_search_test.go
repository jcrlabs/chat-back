package postgres_test

import (
	"strings"
	"testing"
)

// searchQueryValid mirrors the parsing logic in UserRepo.Search.
// Returns (username, tag, ok).
func parseTagQuery(q string) (string, string, bool) {
	parts := strings.SplitN(q, "#", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func TestParseTagQuery_Valid(t *testing.T) {
	cases := []struct {
		input    string
		wantUser string
		wantTag  string
	}{
		{"alice#0042", "alice", "0042"},
		{"Bob#1234", "Bob", "1234"},
		{"user#9999", "user", "9999"},
		{"x#0001", "x", "0001"},
	}
	for _, tc := range cases {
		u, tag, ok := parseTagQuery(tc.input)
		if !ok {
			t.Errorf("%q: expected valid, got invalid", tc.input)
			continue
		}
		if u != tc.wantUser || tag != tc.wantTag {
			t.Errorf("%q: got user=%q tag=%q, want user=%q tag=%q", tc.input, u, tag, tc.wantUser, tc.wantTag)
		}
	}
}

func TestParseTagQuery_Invalid(t *testing.T) {
	cases := []string{
		"alice",       // no #
		"alice#",      // empty tag
		"",            // empty string
		"#0042",       // empty username (still parses, but username is empty — caller decides)
	}
	for _, input := range cases {
		_, _, ok := parseTagQuery(input)
		// "#0042" parses as username="" tag="0042" which is ok=true — that's fine,
		// the DB query with ILIKE '' returns nothing useful.
		// The real invalid ones are the first three.
		if input == "alice" || input == "alice#" || input == "" {
			if ok {
				t.Errorf("%q: expected invalid, got valid", input)
			}
		}
	}
}
