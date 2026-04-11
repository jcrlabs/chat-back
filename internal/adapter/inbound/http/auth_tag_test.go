package http_test

import (
	"regexp"
	"strconv"
	"testing"
)

// isValidTag checks that a tag is a zero-padded 4-digit string in range 0001-9999.
func isValidTag(tag string) bool {
	if len(tag) != 4 {
		return false
	}
	matched, _ := regexp.MatchString(`^\d{4}$`, tag)
	if !matched {
		return false
	}
	n, _ := strconv.Atoi(tag)
	return n >= 1 && n <= 9999
}

func TestTagFormat(t *testing.T) {
	valid := []string{"0001", "0042", "1234", "9999"}
	for _, tag := range valid {
		if !isValidTag(tag) {
			t.Errorf("%q should be valid", tag)
		}
	}

	invalid := []string{"0000", "10000", "123", "12345", "abcd", "", "00a1"}
	for _, tag := range invalid {
		if isValidTag(tag) {
			t.Errorf("%q should be invalid", tag)
		}
	}
}

func TestTagZeroPadding(t *testing.T) {
	// Tags below 1000 must be zero-padded to 4 digits
	cases := map[int]string{
		1:    "0001",
		42:   "0042",
		100:  "0100",
		1000: "1000",
		9999: "9999",
	}
	for n, want := range cases {
		got := zeroPad(n)
		if got != want {
			t.Errorf("zeroPad(%d) = %q, want %q", n, got, want)
		}
	}
}

func zeroPad(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}
