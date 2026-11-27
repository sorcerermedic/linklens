package shortener

import (
	"errors"
	"math"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   uint64
		want string
	}{
		{"zero", 0, "0"},
		{"single digit", 9, "9"},
		{"first lowercase", 10, "a"},
		{"last lowercase", 35, "z"},
		{"first uppercase", 36, "A"},
		{"last uppercase", 61, "Z"},
		{"two digits", 62, "10"},
		{"arbitrary", 238328, "1000"},
		{"large", 3844, "100"},
		{"max uint64", math.MaxUint64, "lYGhA16ahyf"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Encode(tc.in); got != tc.want {
				t.Fatalf("Encode(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    uint64
		wantErr error
	}{
		{"zero", "0", 0, nil},
		{"lowercase", "a", 10, nil},
		{"uppercase", "Z", 61, nil},
		{"multi digit", "10", 62, nil},
		{"arbitrary", "1000", 238328, nil},
		{"max uint64", "lYGhA16ahyf", math.MaxUint64, nil},
		{"empty", "", 0, ErrInvalidCode},
		{"invalid char", "ab-c", 0, ErrInvalidCode},
		{"unicode", "héllo", 0, ErrInvalidCode},
		{"overflow", "lYGhA16ahyg", 0, ErrOverflow},
		{"way too long", "ZZZZZZZZZZZZZZZ", 0, ErrOverflow},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Decode(tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Decode(%q) error = %v, want %v", tc.in, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Fatalf("Decode(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	values := []uint64{0, 1, 61, 62, 63, 3843, 3844, 1 << 20, 1 << 40, math.MaxInt64, math.MaxUint64}
	for _, v := range values {
		got, err := Decode(Encode(v))
		if err != nil {
			t.Fatalf("round trip %d: %v", v, err)
		}
		if got != v {
			t.Fatalf("round trip %d: got %d", v, got)
		}
	}
}

func TestEncodeIsMonotonic(t *testing.T) {

	seen := make(map[string]struct{}, 10000)
	for i := uint64(0); i < 10000; i++ {
		s := Encode(i)
		if _, dup := seen[s]; dup {
			t.Fatalf("duplicate code %q for id %d", s, i)
		}
		seen[s] = struct{}{}
	}
}

func TestIsBase62(t *testing.T) {
	tests := map[string]bool{"": false, "abc": true, "ABC09": true, "a_b": false, "a-b": false, "ü": false}
	for in, want := range tests {
		if got := IsBase62(in); got != want {
			t.Errorf("IsBase62(%q) = %v, want %v", in, got, want)
		}
	}
}
