package main

import (
	"encoding/base64"
	"testing"
)

func TestEmailFromToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"email":"jwt@example.com"}`))
	cases := map[string]string{
		`{"id_token_email":"a@example.com"}`: "a@example.com",
		`{"id_token":"x.` + payload + `.y"}`: "jwt@example.com",
		`{"id_token":"not-a-jwt"}`:           "",
		`not json`:                           "",
		`[]`:                                 "",
		``:                                   "",
	}
	for in, want := range cases {
		if got := emailFromToken([]byte(in)); got != want {
			t.Errorf("emailFromToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseArgs(t *testing.T) {
	p, err := parseArgs([]string{"work", "--login", "-f", "--agy-bin=/x/agy"}, 1,
		map[string]string{"--login": "login", "-f": "force"},
		map[string]string{"--agy-bin": "agy-bin"})
	if err != nil {
		t.Fatal(err)
	}
	if p.pos[0] != "work" || !p.bools["login"] || !p.bools["force"] || p.vals["agy-bin"] != "/x/agy" {
		t.Fatalf("unexpected result: %+v", p)
	}
	if _, err := parseArgs([]string{"--bogus", "x"}, 1, nil, nil); err == nil {
		t.Fatal("expected error for unknown option")
	}
	if _, err := parseArgs(nil, 1, nil, nil); err == nil {
		t.Fatal("expected error for missing NAME")
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"work", "a.b_c-1"} {
		if _, err := validateName(ok); err != nil {
			t.Errorf("validateName(%q) failed: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".hidden", "a/b", "../x"} {
		if _, err := validateName(bad); err == nil {
			t.Errorf("validateName(%q) should fail", bad)
		}
	}
}
