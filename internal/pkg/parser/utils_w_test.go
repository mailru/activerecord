package parser

import (
	"go/ast"
	"reflect"
	"testing"
)

func Test_getNodeName(t *testing.T) {
	type args struct {
		node string
	}
	tests := []struct {
		name            string
		args            args
		wantName        string
		wantPublicName  string
		wantPackageName string
		wantErr         bool
	}{
		{
			name: "error fileds",
			args: args{
				node: "Fields",
			},
			wantName:        "Fields",
			wantPublicName:  "",
			wantPackageName: "",
			wantErr:         true,
		},
		{
			name: "unknown fields",
			args: args{
				node: "bla",
			},
			wantName:        "",
			wantPublicName:  "",
			wantPackageName: "",
			wantErr:         true,
		},
		{
			name: "success fields",
			args: args{
				node: "FieldsUser",
			},
			wantName:        "Fields",
			wantPublicName:  "User",
			wantPackageName: "user",
			wantErr:         false,
		},
		{
			name: "fields object",
			args: args{
				node: "FieldsObjectUser",
			},
			wantName:        "FieldsObject",
			wantPublicName:  "User",
			wantPackageName: "user",
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotPublicName, gotPackageName, err := getNodeName(tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodeName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotName != tt.wantName {
				t.Errorf("getNodeName() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotPublicName != tt.wantPublicName {
				t.Errorf("getNodeName() gotPublicName = %v, want %v", gotPublicName, tt.wantPublicName)
			}
			if gotPackageName != tt.wantPackageName {
				t.Errorf("getNodeName() gotPackageName = %v, want %v", gotPackageName, tt.wantPackageName)
			}
		})
	}
}

func Test_splitParam(t *testing.T) {
	type args struct {
		str  string
		rule map[TagNameType]ParamValueRule
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "emptytag", wantErr: false,
			args: args{
				str:  "",
				rule: map[TagNameType]ParamValueRule{},
			},
			want: [][]string{},
		},
		{
			name: "onetag", wantErr: false,
			args: args{
				str:  "a:b",
				rule: map[TagNameType]ParamValueRule{},
			},
			want: [][]string{{"a", "b"}},
		},
		{
			name: "anytag", wantErr: false,
			args: args{
				str:  "a:b;d:f",
				rule: map[TagNameType]ParamValueRule{},
			},
			want: [][]string{{"a", "b"}, {"d", "f"}},
		},
		{
			name: "anytagempty", wantErr: false,
			args: args{
				str:  "a:b;;d:f",
				rule: map[TagNameType]ParamValueRule{},
			},
			want: [][]string{{"a", "b"}, {"d", "f"}},
		},
		{
			name: "tagisflag", wantErr: false,
			args: args{
				str:  "a:b;d:f;g",
				rule: map[TagNameType]ParamValueRule{"g": ParamNotNeedValue},
			},
			want: [][]string{{"a", "b"}, {"d", "f"}, {"g"}},
		},
		{
			name:    "tagflagerr",
			wantErr: false,
			args: args{
				str:  "a",
				rule: map[TagNameType]ParamValueRule{},
			},
			want: [][]string{{"a"}},
		},
		{
			name: "tagflagerr", wantErr: true,
			args: args{
				str:  "a:b",
				rule: map[TagNameType]ParamValueRule{"a": ParamNotNeedValue},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitParam(tt.args.str, tt.args.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_splitTag(t *testing.T) {
	type args struct {
		field     *ast.Field
		checkFlag uint32
		rule      map[TagNameType]ParamValueRule
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "emptytag", wantErr: false,
			args: args{
				field:     &ast.Field{Tag: &ast.BasicLit{Value: "`ar:\"\"`"}},
				rule:      map[TagNameType]ParamValueRule{},
				checkFlag: NoCheckFlag,
			},
			want: [][]string{},
		},
		{
			name: "emptytagcheck", wantErr: true,
			args: args{
				field:     &ast.Field{Tag: &ast.BasicLit{Value: "`ar:\"\"`"}},
				rule:      map[TagNameType]ParamValueRule{},
				checkFlag: CheckFlagEmpty,
			},
			want: nil,
		},
		{
			name: "tagnoprefix", wantErr: true,
			args: args{
				field:     &ast.Field{Tag: &ast.BasicLit{Value: "dsjfgsadkjgfdskj"}},
				checkFlag: CheckFlagEmpty,
				rule:      map[TagNameType]ParamValueRule{},
			},
			want: nil,
		},
		{
			name: "tag", wantErr: false,
			args: args{
				field:     &ast.Field{Tag: &ast.BasicLit{Value: "`ar:\"a:b;c:d;d:r,t\"`"}},
				rule:      map[TagNameType]ParamValueRule{},
				checkFlag: CheckFlagEmpty,
			},
			want: [][]string{{"a", "b"}, {"c", "d"}, {"d", "r,t"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitTag(tt.args.field, tt.args.checkFlag, tt.args.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkBoolType(t *testing.T) {
	type args struct {
		indType ast.Expr
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "bool", args: args{indType: &ast.Ident{Name: "bool"}}, wantErr: false},
		{name: "bool", args: args{indType: &ast.Ident{Name: "int"}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkBoolType(tt.args.indType); (err != nil) != tt.wantErr {
				t.Errorf("checkBoolType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
