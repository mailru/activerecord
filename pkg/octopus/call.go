package octopus

import (
	"context"

	"github.com/pkg/errors"
)

// CallLua - функция для вызова lua процедур. В будущем надо будет сделать возможность декларативно описывать процедуры в модели
// и в сгенерированном коде вызывать эту функцию.
// Так же надо будет сделать возможность описывать формат для результата в произвольной форме, а не в форме тупла для мочёдели.
func CallLua(ctx context.Context, connection *Connection, name string, args ...string) ([]TupleData, error) {
	w := PackLua(name, args...)

	resp, err := connection.Call(ctx, RequestTypeCall, w)
	if err != nil {
		return []TupleData{}, errors.Wrap(err, "error call lua")
	}

	tuple, err := ProcessResp(resp, 0)
	if err != nil {
		return []TupleData{}, errors.Wrap(err, "error unpack lua response")
	}

	return tuple, nil
}
