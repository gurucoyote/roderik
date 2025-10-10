package cmd

import "testing"

func TestSanitizeProfileDirName(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
		error  bool
	}{
		{name: "empty", input: "", expect: "", error: false},
		{name: "trims whitespace", input: "  Profile 1  ", expect: "Profile 1", error: false},
		{name: "reject path separator", input: "../../etc", expect: "", error: true},
		{name: "reject dot", input: ".", expect: "", error: true},
		{name: "reject colon", input: "C:drive", expect: "", error: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeProfileDirName(tc.input)
			if tc.error {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.expect {
				t.Fatalf("expected %q, got %q", tc.expect, got)
			}
		})
	}
}
