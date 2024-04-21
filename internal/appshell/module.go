package appshell

import (
	"github.com/d5/tengo/v2"
)

type (
	Module struct {
		name string

		tengoModule *tengo.BuiltinModule
	}
)

func NewModule(name string) *Module {
	return &Module{name: name,
		tengoModule: &tengo.BuiltinModule{
			Attrs: make(map[string]tengo.Object),
		},
	}
}

func (m *Module) AddFuncRaw(name string, fn tengo.CallableFunc) {
	m.tengoModule.Attrs[name] = &tengo.BuiltinFunction{Name: name, Value: fn}
}

func (m *Module) AddValue(name string, fn any) bool {
	var err error
	m.tengoModule.Attrs[name], err = tengo.FromInterface(fn)
	return err == nil
}
