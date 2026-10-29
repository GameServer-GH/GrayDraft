package textutil

import "testing"

func TestStripEmojis(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"巴黎学科资料", "巴黎学科资料"},
		{"InternVL 接口介绍 ✅", "InternVL 接口介绍"},
		{"Hello 👋 World", "Hello  World"},
		{"🇫🇷法国", "法国"},
		{"", ""},
	}
	for _, c := range cases {
		got := StripEmojis(c.in)
		if got != c.want {
			t.Fatalf("StripEmojis(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
