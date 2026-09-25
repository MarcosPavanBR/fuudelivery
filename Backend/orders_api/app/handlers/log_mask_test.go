package handlers

import "testing"

func TestMaskPhone(t *testing.T) {
	for in, want := range map[string]string{
		"+5511999998888": "***8888",
		" 11988887777 ":  "***7777",
		"123":            "***",
		"":               "***",
	} {
		if got := maskPhone(in); got != want {
			t.Errorf("maskPhone(%q) = %q, want %q", in, got, want)
		}
	}
}
