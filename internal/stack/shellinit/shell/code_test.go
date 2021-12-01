package shell_test

import (
	"testing"

	"github.com/elastic/elastic-package/internal/stack/shellinit/shell"
)

func TestCodeTemplate(t *testing.T) {
	type args struct {
		s shell.Type
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "bash code template", args: args{s: shell.Bash}, want: shell.BashInitCode},
		{name: "fish code template", args: args{s: shell.Fish}, want: shell.FishInitCode},
		{name: "sh code template", args: args{s: shell.Sh}, want: shell.ShInitCode},
		{name: "zsh code template", args: args{s: shell.Zsh}, want: shell.ZshInitCode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shell.InitTemplate(tt.args.s); got != tt.want {
				t.Errorf("CodeTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}
