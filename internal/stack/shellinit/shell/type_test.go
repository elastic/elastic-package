package shell_test

import (
	"testing"

	"github.com/elastic/elastic-package/internal/stack/shellinit/shell"
)

func TestType_Stringer(t *testing.T) {
	tests := []struct {
		name string
		s    shell.Type
		want string
	}{
		{name: "bash", s: shell.Bash, want: "bash"},
		{name: "fish", s: shell.Fish, want: "fish"},
		{name: "sh", s: shell.Sh, want: "sh"},
		{name: "zsh", s: shell.Zsh, want: "zsh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("Type.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want shell.Type
	}{
		{"bash", args{s: "bash"}, shell.Bash},
		{"fish", args{s: "fish"}, shell.Fish},
		{"zsh", args{s: "zsh"}, shell.Zsh},
		{"sh", args{s: "sh"}, shell.Sh},
		{"default", args{s: "foobar"}, shell.Sh},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shell.FromString(tt.args.s); got != tt.want {
				t.Errorf("FromString() = %v, want %v", got, tt.want)
			}
		})
	}
}
