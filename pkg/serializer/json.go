package serializer

import (
	"encoding/json"
	"fmt"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

func JSONUnmarshal(data string, v any) error {
	err := json.Unmarshal([]byte(data), v)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrUnmarshalJSON, err)
	}

	return nil
}

func JSONMarshal(v any) (string, error) {
	ret, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrMarshalJSON, err)
	}

	return string(ret), nil
}
