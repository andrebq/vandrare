package appshell

import (
	"encoding/json"
	"fmt"
	"io"
)

func EchoModule(w io.Writer, alias string) *Module {
	if alias == "" {
		alias = "echo"
	}
	echoMod := NewModule(alias)
	echoMod.AddFuncRaw("print", DynFuncNR0(func(args ...any) error {
		strs := make([]string, len(args))
		for i, v := range args {
			strs[i] = fmt.Sprintf("%v", v)
		}
		return json.NewEncoder(w).Encode(strs)
	}))
	echoMod.AddFuncRaw("printJSON", DynFuncNR0(func(args ...any) error {
		if len(args) > 1 {
			println("here")
			return json.NewEncoder(w).Encode(args)
		} else if len(args) == 1 {
			println("here!")
			return json.NewEncoder(w).Encode(args[0])
		}
		return nil
	}))

	return echoMod
}
