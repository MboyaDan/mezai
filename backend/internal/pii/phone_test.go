package pii

import "testing"

func TestNormaliseKenyanMobile(t *testing.T) {
	ok := map[string]string{
		"0712345678":       "+254712345678",
		"0112 345 678":     "+254112345678",
		"254712345678":     "+254712345678",
		"+254 712-345-678": "+254712345678",
	}
	for in, want := range ok {
		got, err := NormaliseKenyanMobile(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "12345", "0812345678", "+255712345678", "07123456789", "07123x5678", "712345678"} {
		if got, err := NormaliseKenyanMobile(in); err == nil {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}

func TestMaskPhone(t *testing.T) {
	if got := MaskPhone("+254712345678"); got != "+254 7•• ••• 678" {
		t.Fatalf("got %q", got)
	}
	if MaskPhone("") != "" || MaskPhone("garbage") != "•••" {
		t.Fatal("empty or malformed input")
	}
}
