package appshell

import (
	"fmt"
	"reflect"

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
	return FuncNR1Cast(goFun, FromInterface[R]())
}

func FuncNR1Cast[T ArgType, R any](goFun func(args ...T) (R, error), customCast func(R) (tengo.Object, error)) tengo.CallableFunc {
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
		return customCast(val)
	}
}

func FromInterface[R any]() func(in R) (tengo.Object, error) {
	return func(in R) (tengo.Object, error) { return tengo.FromInterface(in) }
}

// ToFlatMap returns a function that takes a value of type R
// and maps it to a tengo.ImmutableMap object.
//
// Every field of R must be mappable to a tengo.Object (see tengo.FromInterface)
func ToFlatMap[R any]() func(in R) (tengo.Object, error) {
	var zero R
	var isptr bool
	tp := reflect.TypeOf(zero)
	if tp.Kind() == reflect.Pointer {
		isptr = true
		tp = tp.Elem()
	}
	if tp.Kind() != reflect.Struct {
		return func(in R) (tengo.Object, error) { return tengo.UndefinedValue, fmt.Errorf("%T is not a struct", zero) }
	}
	fieldByName := make(map[string]int)
	for i := 0; i < tp.NumField(); i++ {
		fld := tp.Field(i).Name
		fieldByName[fld] = i
	}
	return func(in R) (tengo.Object, error) {
		val := reflect.ValueOf(in)
		if isptr {
			val = val.Elem()
		}
		mp := tengo.ImmutableMap{
			Value: make(map[string]tengo.Object),
		}
		var err error
		for name, idx := range fieldByName {
			mp.Value[name], err = tengo.FromInterface(val.Field(idx).Interface())
			if err != nil {
				return nil, err
			}
		}
		return &mp, nil
	}
}

func FromInterfaceSlice[R any, S ~[]R](iter func(R) (tengo.Object, error)) func(in S) (tengo.Object, error) {
	return func(in S) (tengo.Object, error) {
		ret := make([]tengo.Object, len(in))
		for i, v := range in {
			var err error
			ret[i], err = iter(v)
			if err != nil {
				return tengo.UndefinedValue, err
			}
		}
		return tengo.FromInterface(ret)
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
