package g2

import (
	"strings"
	"unicode"
)

// PadVersion normalizes a version string so it can be compared strictly as strings.
func PadVersion(ver string) string {
	if ver == "" {
		return ""
	}

	var sb strings.Builder
	var numSb strings.Builder

	flushNum := func() {
		if numSb.Len() > 0 {
			// Pad the numeric part to 10 characters
			s := numSb.String()
			for i := 0; i < 10-len(s); i++ {
				sb.WriteByte('0')
			}
			sb.WriteString(s)
			numSb.Reset()
		}
	}

	for i := 0; i < len(ver); i++ {
		c := ver[i]

		// -rX transformation -> +rX
		if c == '-' && i+1 < len(ver) && ver[i+1] == 'r' {
			flushNum()
			sb.WriteString("+r")
			i++ // skip 'r'
			continue
		}

		if unicode.IsDigit(rune(c)) {
			numSb.WriteByte(c)
		} else {
			flushNum()
			sb.WriteByte(c)
		}
	}
	flushNum()

	return sb.String()
}

// SplitVersionOp extracts the operator (>=, <=, >, <, ~, =, ==) and the remaining version string
func SplitVersionOp(queryVersion string) (version string, op string) {
	op = "=="
	v := queryVersion
	if strings.HasPrefix(queryVersion, "==") {
		op = "=="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, ">=") {
		op = ">="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, "<=") {
		op = "<="
		v = queryVersion[2:]
	} else if strings.HasPrefix(queryVersion, ">") {
		op = ">"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "<") {
		op = "<"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "~") {
		op = "~"
		v = queryVersion[1:]
	} else if strings.HasPrefix(queryVersion, "=") {
		op = "=="
		v = queryVersion[1:]
	}
	return v, op
}
