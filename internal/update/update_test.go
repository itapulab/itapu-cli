package update

import "testing"

func TestNewer(t *testing.T) {
	tests := []struct {
		tag, current      string
		newer, comparable bool
	}{
		{"v0.2.0", "0.1.0", true, true},
		{"v0.1.0", "0.1.0", false, true},
		{"v0.1.0", "0.2.0", false, true},
		{"v1.0.0", "0.9.9", true, true},
		{"v0.1.10", "0.1.9", true, true},
		{"v0.2", "0.1.5", true, true},       // short tags compare as v0.2.0
		{"v0.2.0-rc1", "0.1.0", true, true}, // pre-release suffix ignored
		{"v0.2.0", "dev", false, false},     // dev build
		{"v0.2.0", "996e320", false, false}, // git describe fallback
		{"releases", "0.1.0", false, false}, // redirect didn't resolve a tag
		{"", "0.1.0", false, false},
	}
	for _, tt := range tests {
		newer, comparable := Newer(tt.tag, tt.current)
		if newer != tt.newer || comparable != tt.comparable {
			t.Errorf("Newer(%q, %q) = (%v, %v), want (%v, %v)",
				tt.tag, tt.current, newer, comparable, tt.newer, tt.comparable)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("hello")
	// sha256("hello")
	sums := []byte("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  itapu_0.1.0_darwin_arm64.tar.gz\n")
	if err := verifyChecksum("itapu_0.1.0_darwin_arm64.tar.gz", data, sums); err != nil {
		t.Errorf("valid checksum rejected: %v", err)
	}
	if err := verifyChecksum("itapu_0.1.0_darwin_arm64.tar.gz", []byte("tampered"), sums); err == nil {
		t.Error("tampered archive accepted")
	}
	if err := verifyChecksum("itapu_0.1.0_linux_amd64.tar.gz", data, sums); err == nil {
		t.Error("archive with no published checksum accepted")
	}
}
