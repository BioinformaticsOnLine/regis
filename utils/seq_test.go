package utils

import "testing"

func TestFastaRecordID(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"rb_E4.L.2 l=392 c=18433.0", "rb_E4.L.2"},
		{"TRINITY_DN0_c0_g1_i1", "TRINITY_DN0_c0_g1_i1"},
		{"ENST000001\tgene=X", "ENST000001"},
		{"  spaced  ", "spaced"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := fastaRecordID(tc.in); got != tc.want {
			t.Errorf("fastaRecordID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
