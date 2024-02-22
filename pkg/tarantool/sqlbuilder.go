package tarantool

import "strings"

func BuildSQLPredicateIN(fieldname string, fieldCndt int) string {
	if fieldCndt == 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(fieldname) + 2*fieldCndt + 10)
	b.WriteString(" \"" + fieldname + "\" IN (?")

	for i := 0; i < fieldCndt-1; i++ {
		b.WriteString(", ?")
	}

	b.WriteString(")")

	return b.String()
}
