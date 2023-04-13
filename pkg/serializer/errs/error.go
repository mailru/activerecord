package errs

import "errors"

var (
	ErrMarshalJSON            = errors.New("err marshal json")
	ErrUnmarshalJSON          = errors.New("err unmarshal json")
	ErrMapstructureNewDecoder = errors.New("err mapstructure new decoder")
	ErrMapstructureDecode     = errors.New("err mapstructure decode")
	ErrMapstructureEncode     = errors.New("err mapstructure encode")
	ErrPrintfParse            = errors.New("err printf parse")
)
