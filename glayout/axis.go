package glayout

import (
	"strconv"
	"time"
)

// formatAxis renders a time using a d3/strftime-style format string (the subset
// mermaid axisFormat uses: %Y %y %m %d %H %M %S %b %B %a %A %%).
func formatAxis(t time.Time, format string) string {
	if format == "" {
		format = "%Y-%m-%d"
	}
	var b []byte
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b = append(b, format[i])
			continue
		}
		i++
		switch format[i] {
		case 'Y':
			b = append(b, []byte(strconv.Itoa(t.Year()))...)
		case 'y':
			b = append(b, []byte(pad2(t.Year()%100))...)
		case 'm':
			b = append(b, []byte(pad2(int(t.Month())))...)
		case 'd':
			b = append(b, []byte(pad2(t.Day()))...)
		case 'e':
			b = append(b, []byte(strconv.Itoa(t.Day()))...)
		case 'H':
			b = append(b, []byte(pad2(t.Hour()))...)
		case 'M':
			b = append(b, []byte(pad2(t.Minute()))...)
		case 'S':
			b = append(b, []byte(pad2(t.Second()))...)
		case 'b':
			b = append(b, []byte(t.Format("Jan"))...)
		case 'B':
			b = append(b, []byte(t.Format("January"))...)
		case 'a':
			b = append(b, []byte(t.Format("Mon"))...)
		case 'A':
			b = append(b, []byte(t.Format("Monday"))...)
		case '%':
			b = append(b, '%')
		default:
			b = append(b, '%', format[i])
		}
	}
	return string(b)
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
