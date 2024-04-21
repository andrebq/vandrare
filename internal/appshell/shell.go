package appshell

import (
	"context"
	"fmt"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/parser"
	"github.com/d5/tengo/v2/stdlib"
)

type (
	Shell struct {
		modules *tengo.ModuleMap
	}
)

func New(withStdlib bool) *Shell {
	s := &Shell{
		modules: tengo.NewModuleMap(),
	}
	if withStdlib {
		s.modules.AddMap(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
	}
	return s
}

func (s *Shell) AddModules(m ...*Module) {
	for _, v := range m {
		s.modules.Add(v.name, v.tengoModule)
	}
}

func (s *Shell) Eval(ctx context.Context, code string) (any, error) {
	wrapCode := fmt.Sprintf(`output := (func() { %v })()`, code)
	println(wrapCode)
	sc := tengo.NewScript([]byte(wrapCode))
	sc.EnableFileImport(false)
	sc.SetImports(s.modules)
	result, err := sc.RunContext(ctx)
	if err != nil {
		return nil, err
	}
	output := result.Get("output")
	if output.IsUndefined() {
		return nil, nil
	}
	return tengo.ToInterface(output.Object()), nil
}

func (s *Shell) ValidScript(input string) bool {
	fileSet := parser.NewFileSet()
	srcFile := fileSet.AddFile("(main)", -1, len(input))
	p := parser.NewParser(srcFile, []byte(input), nil)
	_, err := p.ParseFile()
	return err == nil
}
