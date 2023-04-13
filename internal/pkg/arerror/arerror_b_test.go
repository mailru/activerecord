package arerror

import (
	"testing"
)

func TestErrorBase(t *testing.T) {
	type args struct {
		errStruct interface{}
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple error",
			args: args{
				errStruct: &ErrGeneratorPkg{
					Name: "TestError",
					Err:  ErrGeneratorBackendUnknown,
				},
			},
			want: `ErrGeneratorPkg Name: ` + "`TestError`" + `; 
	backend unknown`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorBase(tt.args.errStruct); got != tt.want {
				t.Errorf("ErrorBase() = %v, want %v", got, tt.want)
			}
		})
	}
}
