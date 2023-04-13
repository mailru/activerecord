package generator

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
)

var tmplErrRx = regexp.MustCompile(TemplateName + `:(\d+):`)

func getTmplErrorLine(lines []string, tmplerror string) (string, error) {
	lineTmpl := tmplErrRx.FindStringSubmatch(tmplerror)
	if len(lineTmpl) > 1 {
		lineNum, errParse := strconv.ParseInt(lineTmpl[1], 10, 64)
		if errParse != nil {
			return "", arerror.ErrGeneragorGetTmplLine
		} else if len(lines) == 0 {
			return "", arerror.ErrGeneragorEmptyTmplLine
		} else {
			cntline := 3
			startLine := int(lineNum) - cntline - 1
			if startLine < 0 {
				startLine = 0
			}
			stopLine := int(lineNum) + cntline
			if stopLine > int(lineNum) {
				stopLine = int(lineNum)
			}
			errorLines := lines[startLine:stopLine]
			for num := range errorLines {
				if num == cntline {
					errorLines[num] = "-->> " + errorLines[num]
				} else {
					errorLines[num] = "     " + errorLines[num]
				}
			}
			return "\n" + strings.Join(errorLines, ""), nil
		}
	} else {
		return "", arerror.ErrGeneragorErrorLineNotFound
	}
}
