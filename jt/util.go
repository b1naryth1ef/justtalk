package jt

import "strings"
import "unicode/utf8"

func escapeJSONString(s string) string {
	replacerTable := []string{
		`"`, `\"`, `\`, `\\`,
		"\u0000", "\\\u0000", "\u0001", "\\\u0001",
		"\u0002", "\\\u0002", "\u0003", "\\\u0003",
		"\u0004", "\\\u0004", "\u0005", "\\\u0005",
		"\u0006", "\\\u0006", "\u0007", "\\\u0007",
		"\u0008", "\\\u0008", "\u0009", "\\\u0009",
		"\u000A", "\\\u000A", "\u000B", "\\\u000B",
		"\u000C", "\\\u000C", "\u000D", "\\\u000D",
		"\u000E", "\\\u000E", "\u000F", "\\\u000F",
		"\u0010", "\\\u0010", "\u0011", "\\\u0011",
		"\u0012", "\\\u0012", "\u0013", "\\\u0013",
		"\u0014", "\\\u0014", "\u0015", "\\\u0015",
		"\u0016", "\\\u0016", "\u0017", "\\\u0017",
		"\u0018", "\\\u0018", "\u0019", "\\\u0019",
		"\u001A", "\\\u001A", "\u001B", "\\\u001B",
		"\u001C", "\\\u001C", "\u001D", "\\\u001D",
		"\u001E", "\\\u001E", "\u001F", "\\\u001F",
	}
	jsonEscaper := strings.NewReplacer(replacerTable...)
	return `"` + jsonEscaper.Replace(s) + `"`
}

func ParameterizeJSON(q string, params ...string) string {
	const backslash_rune = 92
	const dbl_quote_rune = 34
	const question_rune = 63

	//Track the state of the cursor
	backslash := false
	in_quotes := false
	param_positions := make([]int, 0)
	for i, c := range q {
		if in_quotes {
			if backslash {
				//Ignore the character after a backslash (ie, an escape)
				//Technically we might need to ignore up to 5 for unicode literals
				//But those are themselves valid characters for a string & shouldn't
				//embed anything that changes escaping of subsequent characters
				//Be careful though if you plan on writing invalid JSON in the first
				//place - if you do something like "\u43"?" you might be able to
				//cause a syntax error in a different place
				backslash = false
			} else if c == backslash_rune {
				//Start escaping if we haven't already
				backslash = true
			} else if c == dbl_quote_rune {
				//If we're in a string and not escaping, a quote ends the present string
				in_quotes = false
			}
		} else if c == question_rune {
			//Not in quotes (ie, outside a string) a naked question rune is interpolated
			param_positions = append(param_positions, i)
		} else if c == dbl_quote_rune {
			//Not in quotes, a double quote starts a string
			in_quotes = true
		}
	}

	//Loop again, this time splitting at those positions and inserting the
	//appropriate param
	param_ct := 0
	accumulator := make([]byte, 0, len(q))
	for i, c := range q {
		if len(param_positions) == 0 || i != param_positions[0] {
			//Accumulate from the original into the output buffer
			buf := make([]byte, 8)
			n := utf8.EncodeRune(buf, c)
			buf = buf[:n]
			accumulator = append(accumulator, buf...)
		} else {
			//Interpolate
			interpolate_val := escapeJSONString(params[param_ct])
			//Append it to the output buffer
			accumulator = append(accumulator, []byte(interpolate_val)...)
			//Continue
			param_ct += 1
			param_positions = param_positions[1:]
		}
	}
	return string(accumulator)
}
