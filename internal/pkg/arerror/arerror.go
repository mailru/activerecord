package arerror

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Базовая функция для обработки отображения ошибки
// Ошибки могуть быть бесконечно вложены друг в друга,
// каждая новая вложенная ошибка распечатывается с новой строки
// Сама ошибка это структура с любым набором полей,
// поле Err содержит вложенную ошибку
func ErrorBase(errStruct interface{}) string {
	reflV := reflect.ValueOf(errStruct).Elem()
	reflT := reflV.Type()
	reflT.Kind()
	fmtO := []string{}
	param := []interface{}{}
	for i := 0; i < reflV.NumField(); i++ {
		fieldT := reflT.Field(i)
		fieldV := reflV.Field(i)
		form, ok := fieldT.Tag.Lookup("format")
		if !ok {
			form = "%s"
		}
		if fieldT.Name == "Err" {
			fmtO = append(fmtO, "\n\t"+form)
		} else {
			fmtO = append(fmtO, fieldT.Name+": `"+form+"`")
		}
		param = append(param, fieldV.Interface())
	}
	return fmt.Sprintf(reflT.Name()+" "+strings.Join(fmtO, "; "), param...)
}

var ErrBadPkgName = errors.New("bad package name. See https://go.dev/blog/package-names")
