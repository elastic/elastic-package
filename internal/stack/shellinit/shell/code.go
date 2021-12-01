package shell

import "strings"

// InitTemplate returns code templates for shell initialization
func InitTemplate(s Type) string {
	switch s {
	case Bash:
		return BashInitCode
	case Fish:
		return FishInitCode
	case Sh:
		return ShInitCode
	case Zsh:
		return ZshInitCode
	default:
		panic("shell type is unknown, should be one of " + strings.Join(AvailableShellTypes(), ", "))
	}
}
