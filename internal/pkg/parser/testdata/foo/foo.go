package foo

import "github.com/mailru/activerecord/internal/pkg/parser/testdata/ds"

type Beer struct{}

type Foo struct {
	Key      string
	Bar      ds.AppInfo
	BeerData []Beer `ar:"beer_data"`
	MapData  map[string]any
	Other    map[string]any `ar:",ignore"`
}
