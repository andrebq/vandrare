package appshell

import (
	"fmt"

	"github.com/d5/tengo/v2"
)

type (
	ArgType interface {
		int64 | float64 | []byte | string
	}
)

func DynFuncNR0(goFun func(args ...any) error) tengo.CallableFunc {
	return func(args ...tengo.Object) (tengo.Object, error) {
		iargs := make([]any, len(args))
		for i, v := range args {
			iargs[i] = tengo.ToInterface(v)
		}
		return tengo.UndefinedValue, goFun(iargs...)
	}
}

func FuncNR0[T ArgType](goFun func(args ...T) error) tengo.CallableFunc {
	return func(args ...tengo.Object) (tengo.Object, error) {
		iargs := make([]T, len(args))
		for i, v := range args {
			if ok := cast(&iargs[i], v); !ok {
				return tengo.UndefinedValue, fmt.Errorf("cannot cast from tengo:%v to %T", v.TypeName(), iargs[0])
			}
		}
		return tengo.UndefinedValue, goFun(iargs...)
	}
}

func FuncNR1[T ArgType, R ArgType](goFun func(args ...T) (R, error)) tengo.CallableFunc {
	return func(args ...tengo.Object) (tengo.Object, error) {
		iargs := make([]T, len(args))
		for i, v := range args {
			if ok := cast(&iargs[i], v); !ok {
				return tengo.UndefinedValue, fmt.Errorf("cannot cast from tengo:%v to %T", v.TypeName(), iargs[0])
			}
		}
		val, err := goFun(iargs...)
		if err != nil {
			return tengo.UndefinedValue, err
		}
		return tengo.FromInterface(val)
	}
}

func cast(out any, in tengo.Object) bool {
	var ok bool
	switch out := out.(type) {
	case *int64:
		*out, ok = tengo.ToInt64(in)
	case *string:
		*out, ok = tengo.ToString(in)
	case *float64:
		*out, ok = tengo.ToFloat64(in)
	case *[]byte:
		*out, ok = tengo.ToByteSlice(in)
	}
	return ok
}
