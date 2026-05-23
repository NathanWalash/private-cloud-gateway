package notify

import (
	"testing"
)

func TestHTMLEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a & b", "a &amp; b"},
		{"<>&", "&lt;&gt;&amp;"},
	}
	for _, c := range cases {
		got := htmlEscape(c.in)
		if got != c.want {
			t.Errorf("htmlEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitComma(t *testing.T) {
	got := splitComma("a,b,c")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("splitComma: unexpected result %v", got)
	}
}

func TestTrimSp(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  hello  ", "hello"},
		{"no-spaces", "no-spaces"},
		{"", ""},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := trimSp(c.in); got != c.want {
			t.Errorf("trimSp(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildMIME(t *testing.T) {
	msg := buildMIME("from@example.com", "to@example.com", "Subject", "Body")
	for _, want := range []string{
		"From: from@example.com",
		"To: to@example.com",
		"Subject: Subject",
		"Body",
		"MIME-Version: 1.0",
	} {
		if !contains(msg, want) {
			t.Errorf("MIME message missing %q\n\nGot:\n%s", want, msg)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsRec(s, sub))
}

func containsRec(s, sub string) bool {
	for i := range s {
		if i+len(sub) <= len(s) && s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
