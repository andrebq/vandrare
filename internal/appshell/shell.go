package appshell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

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

func (s *Shell) EvalInteractive(ctx context.Context, stdin io.Reader) error {
	sc := bufio.NewScanner(stdin)
	acc := &strings.Builder{}

	oldVars := []*tengo.Variable{}

	for sc.Scan() {
		fmt.Fprintln(acc, sc.Text())
		if s.ValidScript(acc.String()) {

			sc := tengo.NewScript([]byte(acc.String()))
			sc.EnableFileImport(false)
			sc.SetImports(s.modules)

			acc.Reset()
			for _, v := range oldVars {
				sc.Add(v.Name(), v.Object())
			}
			state, err := sc.RunContext(ctx)
			if err != nil {
				return err
			}
			oldVars = append(oldVars[:0], state.GetAll()...)
		}
	}
	ret := make(map[string]any)
	for _, v := range oldVars {
		ret[v.Name()] = v.Value()
	}
	return nil
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
