package appshell_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/andrebq/vandrare/internal/appshell"
	"github.com/d5/tengo/v2"
)

func TestSimple(t *testing.T) {

	shell := appshell.New(false)
	logMod := appshell.NewModule("log")
	var msg string
	logMod.AddFuncRaw("info", appshell.DynFuncNR0(func(args ...any) error {
		msg = fmt.Sprint(args...)
		return nil
	}))
	appMod := appshell.NewModule("salute")
	appMod.AddValue("name", "alice")
	appMod.AddFuncRaw("salute", func(args ...tengo.Object) (tengo.Object, error) {
		return tengo.FromInterface(fmt.Sprintf("Hello: %v", tengo.ToInterface(args[0])))
	})
	shell.AddModules(logMod, appMod)
	output, err := shell.Eval(context.Background(), `
		log := import("log")
		salute := import("salute")
		log.info(salute.salute(salute.name))

		return salute.salute("bob")
	`)
	if err != nil {
		t.Fatal(err)
	} else if output, ok := output.(string); !ok {
		t.Fatalf("output should be a string but got: %#v", output)
	} else if output != "Hello: bob" {
		t.Fatal("Output does not match expected outcome", output)
	}

	if msg != "Hello: alice" {
		t.Fatal("msg does not match expected outcome")
	}
}
