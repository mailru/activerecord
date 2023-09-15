package parser

import (
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

type Foo struct {
	Bar    int    `ar:"bar"`
	Other  string `ar:"bar,ignore"`
	Other2 string `ar:",ignore"`
}

func TestParsePartialStructFields(t *testing.T) {
	type args struct {
		dst     *ds.RecordPackage
		name    string
		pkgName string
		path    string
	}

	dst := ds.NewRecordPackage()

	if _, err := dst.AddImport("github.com/mailru/activerecord/internal/pkg/parser"); err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	tests := []struct {
		name    string
		args    args
		want    []ds.PartialFieldDeclaration
		wantErr bool
	}{
		{
			name: "parse fields of parser.Foo struct",
			args: args{
				dst:     dst,
				name:    "Foo",
				pkgName: "parser",
				path:    ".",
			},
			want: []ds.PartialFieldDeclaration{
				{Name: "Bar", Type: "int", MappingKeyName: "bar"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePartialStructFields(tt.args.dst, tt.args.name, tt.args.pkgName, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePartialStructFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParsePartialStructFields() got = %v, want %v", got, tt.want)
			}
		})
	}
}
