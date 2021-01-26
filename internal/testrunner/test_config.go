package testrunner

import "net/url"

type SkipConfig struct {
	Reason string  `config:"reason"`
	Link   url.URL `config:"url"`
}
