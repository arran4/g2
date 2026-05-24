package main

import (
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
)

func runWorldTUI(path string, lines []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal")
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	cursor := 0
	mode := "normal" // "normal" or "insert"
	inputBuffer := ""
	scrollOffset := 0

	render := func() {
		// Clear screen and reset cursor
		fmt.Print("\033[2J\033[H")

		fmt.Print("Manage Portage World List\r\n")
		fmt.Print("q: quit | s: save | j/k: down/up | c/Space: toggle comment | d: delete | a: add\r\n")
		fmt.Print(strings.Repeat("-", 60) + "\r\n")

		termWidth, termHeight, err := term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			termHeight = 24
			termWidth = 80
		}
		_ = termWidth

		// 3 header lines, 1 or 2 footer lines depending on mode
		listHeight := termHeight - 4
		if mode == "insert" {
			listHeight -= 2
		}

		if cursor < scrollOffset {
			scrollOffset = cursor
		} else if cursor >= scrollOffset + listHeight {
			scrollOffset = cursor - listHeight + 1
		}

		for i, line := range lines {
			if i < scrollOffset || i >= scrollOffset + listHeight {
				continue
			}
			if i == cursor {
				fmt.Print(" > ")
			} else {
				fmt.Print("   ")
			}

			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				fmt.Printf("\033[32m%s\033[0m\r\n", line) // Green for comments
			} else {
				fmt.Printf("%s\r\n", line)
			}
		}

		if mode == "insert" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("New entry: %s_\r\n", inputBuffer)
		}
	}

	render()

	buf := make([]byte, 32)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if mode == "insert" {
			// Handle insert mode
			// Support UTF-8 multi-byte sequences by converting the entire buffer read
			for i := 0; i < n; i++ {
				c := buf[i]
				switch c {
				case 27: // Esc
					mode = "normal"
					inputBuffer = ""
				case 13: // Enter
					if strings.TrimSpace(inputBuffer) != "" {
						if cursor >= len(lines) {
							lines = append(lines, inputBuffer)
						} else {
							// Add to lines and move cursor
							lines = append(lines, "")
							copy(lines[cursor+1:], lines[cursor:])
							lines[cursor] = inputBuffer
						}
					}
					mode = "normal"
					inputBuffer = ""
				case 127, 8: // Backspace
					// Basic backspace handling (assumes 1 byte = 1 char for simplicity in deletion,
					// real UTF-8 backspace requires rune parsing, but this handles standard ASCII well enough
					// without getting overly complex for this TUI)
					if len(inputBuffer) > 0 {
						inputBuffer = inputBuffer[:len(inputBuffer)-1]
					}
				default:
					// Just append printable/valid characters directly from the buffer.
					// This allows pasting and UTF-8 to work naturally since they arrive in the buffer together.
					if c >= 32 || c > 127 { // Allows ASCII and extended UTF-8 bytes
						inputBuffer += string(c)
					}
				}
			}
		} else {
			// Handle normal mode
			if n == 1 {
				switch buf[0] {
				case 'q', 27: // q or Esc
					return nil
				case 's', 13: // s or Enter
					return writeWorldFile(path, lines)
				case 'j':
					if cursor < len(lines) {
						cursor++
					}
				case 'k':
					if cursor > 0 {
						cursor--
					}
				case 'c', ' ': // Toggle comment
					if len(lines) > 0 && cursor < len(lines) {
						line := lines[cursor]
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "#") {
							// Uncomment (remove first occurrence of # and leading spaces)
							lines[cursor] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
						} else {
							// Comment
							lines[cursor] = "# " + line
						}
					}
				case 'd': // Delete
					if len(lines) > 0 && cursor < len(lines) {
						lines = append(lines[:cursor], lines[cursor+1:]...)
						if cursor >= len(lines) && cursor > 0 {
							cursor--
						}
					}
				case 'a': // Add
					mode = "insert"
				}
			} else if n >= 3 && buf[0] == 27 && buf[1] == 91 {
				// Handle arrow keys
				switch buf[2] {
				case 'A': // Up
					if cursor > 0 {
						cursor--
					}
				case 'B': // Down
					if cursor < len(lines) {
						cursor++
					}
				}
			}
		}
		render()
	}

	return nil
}
