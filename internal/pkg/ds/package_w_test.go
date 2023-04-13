package ds

import (
	"testing"
)

func Test_getImportName(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "simple getImportName",
			args:    args{path: "go/ast"},
			want:    "ast",
			wantErr: false,
		},
		{
			name:    "small package path",
			args:    args{path: "ast"},
			want:    "ast",
			wantErr: false,
		},
		{
			name:    "gitlab package",
			args:    args{path: "github.com/mailru/activerecord/internal/pkg/arerror"},
			want:    "arerror",
			wantErr: false,
		},
		{
			name:    "import with quote",
			args:    args{path: `error "github.com/mailru/activerecord/internal/pkg/arerror"`},
			want:    "arerror",
			wantErr: false,
		},
		{
			name:    "empty import",
			args:    args{path: ""},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getImportName(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getImportName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getImportName() = %v, want %v", got, tt.want)
			}
		})
	}
}
