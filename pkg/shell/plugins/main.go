package main

import (
	"context"
	"sync"

	"github.com/elastic/elastic-package/pkg/shell"
	"github.com/hashicorp/go-plugin"
)

type ctxKey string

const (
	ctxKeyPackages ctxKey = "Shell.Packages"
	ctxKeyDB       ctxKey = "Shell.DB"
)

type Plugin struct {
	m    sync.Mutex
	cmds map[string]shell.Command
	ctx  context.Context
}

func NewPlugin() *Plugin {
	return &Plugin{
		cmds: map[string]shell.Command{},
		ctx:  context.Background(),
	}
}

func (p *Plugin) Commands() map[string]shell.Command {
	return p.cmds
}

func (p *Plugin) AddValueToCtx(k, v any) {
	p.m.Lock()
	p.ctx = context.WithValue(p.ctx, k, v)
	p.m.Unlock()
}

func (p *Plugin) GetValueFromCtx(k any) any {
	p.m.Lock()
	defer p.m.Unlock()
	return p.ctx.Value(k)
}

func (p *Plugin) RegisterCommand(cmd shell.Command) {
	p.m.Lock()
	p.cmds[cmd.Name()] = cmd
	p.m.Unlock()
}

func main() {
	p := NewPlugin()

	registerChangelogCmd(p)
	registerInitdbCmd(p)
	registerWhereCmd(p)
	registerWritefileCmd(p)
	registerRunscriptCmd(p)
	registerYqCmd(p)

	// pluginMap is the map of plugins we can dispense.
	pluginMap := map[string]plugin.Plugin{
		"shell_plugin": &shell.ShellPlugin{Impl: p},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: shell.Handshake(),
		Plugins:         pluginMap,
	})
}
