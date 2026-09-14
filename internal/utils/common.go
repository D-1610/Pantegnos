package utils

import (
	"fmt"
	"strings"

	"github.com/mazznoer/colorgrad"
)

func ColorizeGradientText(text string, grad colorgrad.Gradient) string {
	var colorized strings.Builder
	text = strings.ReplaceAll(text, "\r\n", "\n")

	if strings.Contains(text, "\n") {
		for line := range strings.SplitSeq(text, "\n") {
			colorized.WriteString(ColorizeGradientText(line, grad) + "\r\n")
		}
		return strings.TrimSuffix(colorized.String(), "\r\n")
	}

	runes := []rune(text)
	length := len(runes)
	for i := range length {
		color := grad.At(float64(i) / float64(length-1))
		red, green, blue, _ := color.RGBA255()
		colorized.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm%c\x1b[0m", red, green, blue, runes[i]))
	}
	return colorized.String()
}
