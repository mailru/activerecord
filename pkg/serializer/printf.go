package serializer

import (
	"fmt"
	"strconv"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

func PrintfUnmarshal(opt string, data string, v *float64) error {
	f, err := strconv.ParseFloat(data, 64)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrPrintfParse, err)
	}

	*v = f

	return nil
}

func PrintfMarshal(opt string, data float64) (string, error) {
	return fmt.Sprintf(opt, data), nil
}
