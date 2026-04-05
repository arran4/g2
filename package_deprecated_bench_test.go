package g2

import (
	"strings"
	"testing"
)

var sampleText = strings.Repeat("This is a sample text that we will be wrapping around with a specific width in order to benchmark the wrapText function. ", 10)

func BenchmarkWrapText_Old(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = wrapTextOld(sampleText, 70)
	}
}

func BenchmarkWrapText_New(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = wrapTextNew(sampleText, 70)
	}
}

func wrapTextOld(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) > width {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}
	lines = append(lines, currentLine)
	return lines
}

func wrapTextNew(text string, width int) []string {
	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	var currentLine strings.Builder
	currentLine.WriteString(words[0])

	for _, word := range words[1:] {
		if currentLine.Len()+1+len(word) > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			currentLine.WriteByte(' ')
			currentLine.WriteString(word)
		}
	}
	lines = append(lines, currentLine.String())
	return lines
}
