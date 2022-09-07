package stack

import (
	"testing"
)

func TestCodeTemplate(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"bash code template", args{"bash"}, posixTemplate},
		{"fish code template", args{"fish"}, fishTemplate},
		{"sh code template", args{"sh"}, posixTemplate},
		{"zsh code template", args{"zsh"}, posixTemplate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := initTemplate(tt.args.s); got != tt.want {
				t.Errorf("CodeTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodeTemplate_wrongInput(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("initTemplate should have paniced here")
		}
	}()

	initTemplate("invalid shell type")
}
