package serializer

import (
	"encoding/json"
	"fmt"

	"github.com/mailru/mapstructure"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

func MapstructureUnmarshal(data string, v any) error {
	m := make(map[string]interface{})

	err := json.Unmarshal([]byte(data), &m)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrUnmarshalJSON, err)
	}

	config := &mapstructure.DecoderConfig{
		// Включает режим, при котором если какое-то поле не использовалось при декодировании, то возращается ошибка
		ErrorUnused: true,
		// Включает режим перезатирания, то есть при декодировании поля в целевой структуре сбрасываются до default value
		// По умолчанию mapstructure пытается смержить 2 объекта
		ZeroFields: true,
		Result:     v,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrMapstructureNewDecoder, err)
	}

	err = decoder.Decode(m)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrMapstructureDecode, err)
	}

	return nil
}

func MapstructureWeakUnmarshal(data string, v any) error {
	var m map[string]interface{}

	err := json.Unmarshal([]byte(data), &m)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrUnmarshalJSON, err)
	}

	config := &mapstructure.DecoderConfig{
		// Включает режим, при котором если какое-то поле не использовалось при декодировании, то возращается ошибка
		ErrorUnused: true,
		// Включает режим перезатирания, то есть при декодировании поля в целевой структуре сбрасываются до default value
		// По умолчанию mapstructure пытается смержить 2 объекта
		ZeroFields: true,
		// Включает режим, нестрогой типизации
		WeaklyTypedInput: true,
		Result:           v,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrMapstructureNewDecoder, err)
	}

	err = decoder.Decode(m)
	if err != nil {
		return fmt.Errorf("%w: %v", errs.ErrMapstructureDecode, err)
	}

	return nil
}

func MapstructureMarshal(v any) (string, error) {
	m := make(map[string]interface{})

	err := mapstructure.Decode(v, &m)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrMapstructureEncode, err)
	}

	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("%w: %v", errs.ErrMarshalJSON, err)
	}

	return string(b), nil
}
