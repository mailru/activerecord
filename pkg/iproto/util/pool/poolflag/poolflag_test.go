package poolflag

import (
	"flag"
	"reflect"
	"testing"
)

func TestFlagSetWrapperFloat64Slice(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		def  []float64
		exp  []float64
		err  bool
	}{
		{
			def: []float64{1, 2, 3},
			exp: []float64{1, 2, 3},
		},
		{
			def:  []float64{1, 2, 3},
			args: []string{"-slice=3,4,5"},
			exp:  []float64{3, 4, 5},
		},
		{
			def:  []float64{1, 2, 3},
			args: []string{"-slice= 3.14, 3.15  "},
			exp:  []float64{3.14, 3.15},
		},
		{
			def:  []float64{1, 2, 3},
			exp:  []float64{1, 2, 3},
			args: []string{"-slice=3.x"},
			err:  true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := flag.NewFlagSet("test", flag.ContinueOnError)
			w := flagSetWrapper{f}

			v := w.Float64Slice("slice", test.def, "description")

			err := f.Parse(test.args)
			if test.err && err == nil {
				t.Errorf("unexpected nil error")
			}
			if !test.err && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if act, exp := *v, test.exp; !reflect.DeepEqual(act, exp) {
				t.Errorf("unexpected value: %v; want %v", act, exp)
			}
		})
	}
}
