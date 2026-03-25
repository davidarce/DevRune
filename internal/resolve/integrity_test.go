package resolve

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestHashBytes verifies that HashBytes produces the correct SHA256 hash format.
func TestHashBytes(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "known ASCII input",
			input: []byte("hello world"),
		},
		{
			name:  "empty input",
			input: []byte{},
		},
		{
			name:  "binary input",
			input: []byte{0x00, 0x01, 0x02, 0xFF, 0xFE},
		},
		{
			name:  "large input",
			input: []byte(strings.Repeat("a", 100_000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashBytes(tt.input)

			// Must start with "sha256:"
			if !strings.HasPrefix(got, "sha256:") {
				t.Errorf("HashBytes() = %q, want prefix %q", got, "sha256:")
			}

			// The hex portion must match a direct sha256 computation.
			sum := sha256.Sum256(tt.input)
			want := fmt.Sprintf("sha256:%x", sum)
			if got != want {
				t.Errorf("HashBytes() = %q, want %q", got, want)
			}

			// The hex portion must be exactly 64 characters (256 bits / 4 bits per hex char).
			hex := strings.TrimPrefix(got, "sha256:")
			if len(hex) != 64 {
				t.Errorf("HashBytes() hex part length = %d, want 64", len(hex))
			}
		})
	}
}

// TestHashBytes_Deterministic verifies that identical inputs always produce identical hashes.
func TestHashBytes_Deterministic(t *testing.T) {
	input := []byte("deterministic test input")

	h1 := HashBytes(input)
	h2 := HashBytes(input)
	h3 := HashBytes(input)

	if h1 != h2 || h2 != h3 {
		t.Errorf("HashBytes is not deterministic: %q, %q, %q", h1, h2, h3)
	}
}

// TestHashBytes_DifferentInputs verifies that different inputs produce different hashes.
func TestHashBytes_DifferentInputs(t *testing.T) {
	h1 := HashBytes([]byte("input one"))
	h2 := HashBytes([]byte("input two"))

	if h1 == h2 {
		t.Errorf("HashBytes produced identical hashes for different inputs: %q", h1)
	}
}

// TestHashBytes_KnownValue verifies the hash of a well-known input against the expected SHA256.
func TestHashBytes_KnownValue(t *testing.T) {
	// SHA256("") = e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	emptyHash := HashBytes([]byte{})
	wantEmpty := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if emptyHash != wantEmpty {
		t.Errorf("HashBytes(empty) = %q, want %q", emptyHash, wantEmpty)
	}
}
