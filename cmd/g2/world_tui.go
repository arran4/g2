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
	mode := "normal" // "normal", "insert", or "filter"
	inputBuffer := ""
	filterQuery := ""
	scrollOffset := 0

	var filteredIndices []int
	var listHeight int

	render := func() {
		// Clear screen and reset cursor
		fmt.Print("\033[2J\033[H")

		fmt.Print("Manage Portage World List\r\n")
		fmt.Print("q: quit | s: save | j/k: down/up | c/Space: toggle comment | d: delete | a: add | /: filter\r\n")
		fmt.Print(strings.Repeat("-", 60) + "\r\n")

		termWidth, termHeight, err := term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			termHeight = 24
			termWidth = 80
		}
		_ = termWidth

		// 3 header lines, 1 or 2 footer lines depending on mode
		listHeight = termHeight - 4
		if mode == "insert" || mode == "filter" || filterQuery != "" {
			listHeight -= 2
		}
		if listHeight < 1 {
			listHeight = 1
		}

		filteredIndices = nil
		filterQueryLower := strings.ToLower(filterQuery)
		for i, line := range lines {
			if filterQuery == "" || strings.Contains(strings.ToLower(line), filterQueryLower) {
				filteredIndices = append(filteredIndices, i)
			}
		}

		if cursor >= len(filteredIndices) && len(filteredIndices) > 0 {
			cursor = len(filteredIndices) - 1
		} else if len(filteredIndices) == 0 {
			cursor = 0
		}

		if cursor < scrollOffset {
			scrollOffset = cursor
		} else if cursor >= scrollOffset+listHeight {
			scrollOffset = cursor - listHeight + 1
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}

		for i, realIdx := range filteredIndices {
			if i < scrollOffset || i >= scrollOffset+listHeight {
				continue
			}
			if i == cursor {
				fmt.Print(" > ")
			} else {
				fmt.Print("   ")
			}

			line := lines[realIdx]
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				fmt.Printf("\033[32m%s\033[0m\r\n", line) // Green for comments
			} else {
				fmt.Printf("%s\r\n", line)
			}
		}

		if mode == "insert" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("New entry: %s_\r\n", inputBuffer)
		} else if mode == "filter" || filterQuery != "" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			if mode == "filter" {
				fmt.Printf("Filter: %s_\r\n", filterQuery)
			} else {
				fmt.Printf("Filter: %s\r\n", filterQuery)
			}
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

		if mode == "insert" || mode == "filter" {
			// Handle insert/filter mode
			// Support UTF-8 multi-byte sequences by converting the entire buffer read
			for i := 0; i < n; i++ {
				c := buf[i]
				switch c {
				case 27: // Esc
					if mode == "insert" {
						inputBuffer = ""
					}
					mode = "normal"
				case 13: // Enter
					if mode == "insert" {
						if strings.TrimSpace(inputBuffer) != "" {
							if len(filteredIndices) == 0 || cursor >= len(filteredIndices) {
								lines = append(lines, inputBuffer)
							} else {
								realIdx := filteredIndices[cursor]
								// Add to lines and move cursor
								lines = append(lines, "")
								copy(lines[realIdx+1:], lines[realIdx:])
								lines[realIdx] = inputBuffer
							}
						}
						inputBuffer = ""
					}
					mode = "normal"
				case 127, 8: // Backspace
					switch mode {
					case "insert":
						runes := []rune(inputBuffer)
						if len(runes) > 0 {
							inputBuffer = string(runes[:len(runes)-1])
						}
					case "filter":
						runes := []rune(filterQuery)
						if len(runes) > 0 {
							filterQuery = string(runes[:len(runes)-1])
							cursor = 0
						}
					}
				default:
					if c >= 32 || c > 127 { // Allows ASCII and extended UTF-8 bytes
						switch mode {
						case "insert":
							inputBuffer += string(c)
						case "filter":
							filterQuery += string(c)
							cursor = 0
						}
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
					if cursor < len(filteredIndices)-1 {
						cursor++
					}
				case 'k':
					if cursor > 0 {
						cursor--
					}
				case 'c', ' ': // Toggle comment
					if len(filteredIndices) > 0 && cursor < len(filteredIndices) {
						realIdx := filteredIndices[cursor]
						line := lines[realIdx]
						trimmed := strings.TrimSpace(line)
						if strings.HasPrefix(trimmed, "#") {
							// Uncomment (remove first occurrence of # and leading spaces)
							lines[realIdx] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
						} else {
							// Comment
							lines[realIdx] = "# " + line
						}
					}
				case 'd': // Delete
					if len(filteredIndices) > 0 && cursor < len(filteredIndices) {
						realIdx := filteredIndices[cursor]
						lines = append(lines[:realIdx], lines[realIdx+1:]...)
						if cursor >= len(filteredIndices)-1 && cursor > 0 {
							cursor--
						}
					}
				case 'a': // Add
					mode = "insert"
				case '/': // Filter
					mode = "filter"
				}
			} else if n >= 3 && buf[0] == 27 && buf[1] == 91 {
				// Handle escape sequences
				switch buf[2] {
				case 'A': // Up arrow
					if cursor > 0 {
						cursor--
					}
				case 'B': // Down arrow
					if cursor < len(filteredIndices)-1 {
						cursor++
					}
				case '5': // Page Up
					if n >= 4 && buf[3] == '~' {
						cursor -= listHeight
						if cursor < 0 {
							cursor = 0
						}
					}
				case '6': // Page Down
					if n >= 4 && buf[3] == '~' {
						cursor += listHeight
						if cursor >= len(filteredIndices) && len(filteredIndices) > 0 {
							cursor = len(filteredIndices) - 1
						}
					}
				case '1', '7': // Home
					if n >= 4 && buf[3] == '~' {
						cursor = 0
					}
				case '4', '8': // End
					if n >= 4 && buf[3] == '~' {
						if len(filteredIndices) > 0 {
							cursor = len(filteredIndices) - 1
						}
					}
				case 'H': // Home (some terminals)
					cursor = 0
				case 'F': // End (some terminals)
					if len(filteredIndices) > 0 {
						cursor = len(filteredIndices) - 1
					}
				}
			} else if n >= 3 && buf[0] == 27 && buf[1] == 'O' {
				// Handle SS3 escape sequences
				switch buf[2] {
				case 'H': // Home
					cursor = 0
				case 'F': // End
					if len(filteredIndices) > 0 {
						cursor = len(filteredIndices) - 1
					}
				}
			}
		}
		render()
	}

	return nil
}
