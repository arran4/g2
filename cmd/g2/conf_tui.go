package main

import (
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
)

func runConfTUI(path string, lines []string, title string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("stdin is not a terminal")
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	cursor := 0
	mode := "normal" // "normal", "insert", "filter", "edit_modal", "prompt_comment_after", "prompt_comment_before", "prompt_replace"
	inputBuffer := ""
	filterQuery := ""
	scrollOffset := 0
	editModalCursor := 0
	promptRealIdx := 0

	editModalOptions := []string{
		"Comment / Uncomment",
		"Add / remove comment (After on line)",
		"Add / remove comment (Line before)",
		"Modify / replace line",
	}

	var filteredIndices []int
	var listHeight int

	render := func() {
		// Clear screen and reset cursor
		fmt.Print("\033[2J\033[H")

		fmt.Printf("Manage %s\r\n", title)
		fmt.Print("q: quit | s: save | j/k: down/up | c/Space: toggle comment | d: delete | a: add | e: edit | /: filter\r\n")
		fmt.Print(strings.Repeat("-", 60) + "\r\n")

		termWidth, termHeight, err := term.GetSize(int(os.Stdin.Fd()))
		if err != nil {
			termHeight = 24
			termWidth = 80
		}
		_ = termWidth

		// 3 header lines, 1 or 2 footer lines depending on mode
		listHeight = termHeight - 4
		if mode == "insert" || mode == "filter" || strings.HasPrefix(mode, "prompt_") || filterQuery != "" {
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
			if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.HasPrefix(strings.TrimSpace(line), "[") {
				fmt.Printf("\033[32m%s\033[0m\r\n", line) // Green for comments and sections
			} else {
				fmt.Printf("%s\r\n", line)
			}
		}

		// Pad remaining lines
		for i := len(filteredIndices); i < scrollOffset+listHeight; i++ {
			fmt.Print("\r\n")
		}

		fmt.Print(strings.Repeat("-", 60) + "\r\n")

		if filterQuery != "" && mode != "filter" {
			fmt.Printf("Filter: %s\r\n", filterQuery)
		}

		if mode == "insert" {
			fmt.Printf("Add: %s", inputBuffer)
		} else if mode == "filter" {
			fmt.Printf("Filter: %s", filterQuery)
		} else if mode == "prompt_comment_after" {
			fmt.Printf("Comment text: %s", inputBuffer)
		} else if mode == "prompt_comment_before" {
			fmt.Printf("Comment text: %s", inputBuffer)
		} else if mode == "prompt_replace" {
			fmt.Printf("Replace line: %s", inputBuffer)
		}

		// Render Edit Modal overlay
		if mode == "edit_modal" {
			modalWidth := 40
			modalHeight := len(editModalOptions) + 2
			startX := (termWidth - modalWidth) / 2
			startY := (termHeight - modalHeight) / 2

			if startX < 0 {
				startX = 0
			}
			if startY < 0 {
				startY = 0
			}

			for i := 0; i < modalHeight; i++ {
				fmt.Printf("\033[%d;%dH", startY+i, startX)
				if i == 0 || i == modalHeight-1 {
					fmt.Print("+" + strings.Repeat("-", modalWidth-2) + "+")
				} else {
					optIdx := i - 1
					prefix := "   "
					if optIdx == editModalCursor {
						prefix = " > "
					}
					text := prefix + editModalOptions[optIdx]
					padding := modalWidth - 2 - len(text)
					if padding < 0 {
						padding = 0
						text = text[:modalWidth-2]
					}
					fmt.Print("|" + text + strings.Repeat(" ", padding) + "|")
				}
			}
			// Reset cursor position to avoid flickering around the modal
			fmt.Printf("\033[%d;%dH", termHeight, 1)
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

		if mode == "edit_modal" {
			if n == 1 {
				switch buf[0] {
				case 27, 'q': // Esc or q
					mode = "normal"
				case 'j':
					if editModalCursor < len(editModalOptions)-1 {
						editModalCursor++
					}
				case 'k':
					if editModalCursor > 0 {
						editModalCursor--
					}
				case 13: // Enter
					if len(filteredIndices) > 0 && cursor < len(filteredIndices) {
						promptRealIdx = filteredIndices[cursor]
						line := lines[promptRealIdx]
						switch editModalCursor {
						case 0: // Comment / Uncomment
							trimmed := strings.TrimSpace(line)
							if strings.HasPrefix(trimmed, "#") {
								lines[promptRealIdx] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
							} else {
								lines[promptRealIdx] = "# " + line
							}
							mode = "normal"
						case 1: // Add / remove comment (After on line)
							trimmed := strings.TrimSpace(line)
							if strings.HasPrefix(trimmed, "#") {
								inputBuffer = ""
								mode = "prompt_comment_after"
							} else if idx := strings.Index(line, "#"); idx != -1 {
								lines[promptRealIdx] = strings.TrimSpace(line[:idx])
								mode = "normal"
							} else {
								inputBuffer = ""
								mode = "prompt_comment_after"
							}
						case 2: // Add / remove comment (Line before)
							if promptRealIdx > 0 && strings.HasPrefix(strings.TrimSpace(lines[promptRealIdx-1]), "#") {
								lines = append(lines[:promptRealIdx-1], lines[promptRealIdx:]...)
								mode = "normal"
							} else {
								// Insert empty line before
								lines = append(lines, "")
								copy(lines[promptRealIdx+1:], lines[promptRealIdx:])
								lines[promptRealIdx] = ""
								inputBuffer = ""
								mode = "prompt_comment_before"
							}
						case 3: // Modify / replace line
							inputBuffer = line
							mode = "prompt_replace"
						}
					} else {
						mode = "normal"
					}
				}
			} else if n >= 3 && buf[0] == 27 && buf[1] == 91 {
				switch buf[2] {
				case 'A': // Up
					if editModalCursor > 0 {
						editModalCursor--
					}
				case 'B': // Down
					if editModalCursor < len(editModalOptions)-1 {
						editModalCursor++
					}
				}
			}
		} else if mode == "normal" {
			if n == 1 {
				switch buf[0] {
				case 'q':
					return nil
				case 's':
					err := writeConfFile(path, lines)
					if err != nil {
						return fmt.Errorf("saving %s: %w", path, err)
					}
					return nil
				case 'j':
					if cursor < len(filteredIndices)-1 {
						cursor++
					}
				case 'k':
					if cursor > 0 {
						cursor--
					}
				case 'c', ' ': // Toggle comment
					if len(filteredIndices) > 0 {
						idx := filteredIndices[cursor]
						trimmed := strings.TrimSpace(lines[idx])
						if strings.HasPrefix(trimmed, "#") {
							lines[idx] = strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
						} else {
							lines[idx] = "# " + lines[idx]
						}
					}
				case 'd': // Delete line
					if len(filteredIndices) > 0 {
						idx := filteredIndices[cursor]
						lines = append(lines[:idx], lines[idx+1:]...)
						if cursor >= len(filteredIndices)-1 && cursor > 0 {
							cursor--
						}
					}
				case 'a':
					mode = "insert"
					inputBuffer = ""
				case 'e':
					if len(filteredIndices) > 0 {
						mode = "edit_modal"
						editModalCursor = 0
					}
				case '/':
					mode = "filter"
				}
			} else if n >= 3 && buf[0] == 27 && buf[1] == 91 {
				switch buf[2] {
				case 'A': // Up
					if cursor > 0 {
						cursor--
					}
				case 'B': // Down
					if cursor < len(filteredIndices)-1 {
						cursor++
					}
				}
			}
		} else if mode == "insert" || mode == "filter" || strings.HasPrefix(mode, "prompt_") {
			if n == 1 {
				switch buf[0] {
				case 27: // Esc
					if mode == "filter" {
						filterQuery = "" // Reset filter on escape
					} else if mode == "prompt_comment_before" {
						// Remove the empty line we added
						lines = append(lines[:promptRealIdx], lines[promptRealIdx+1:]...)
					}
					mode = "normal"
				case 127, 8: // Backspace
					if mode == "filter" {
						if len(filterQuery) > 0 {
							filterQuery = filterQuery[:len(filterQuery)-1]
						}
					} else {
						if len(inputBuffer) > 0 {
							inputBuffer = inputBuffer[:len(inputBuffer)-1]
						}
					}
				case 13: // Enter
					if mode == "insert" {
						if strings.TrimSpace(inputBuffer) != "" {
							if len(filteredIndices) > 0 && cursor < len(filteredIndices) {
								idx := filteredIndices[cursor] + 1
								lines = append(lines, "")
								copy(lines[idx+1:], lines[idx:])
								lines[idx] = inputBuffer
								cursor++
							} else {
								lines = append(lines, inputBuffer)
								cursor = len(lines) - 1
							}
						}
					} else if mode == "prompt_comment_after" {
						if strings.TrimSpace(inputBuffer) != "" {
							lines[promptRealIdx] = lines[promptRealIdx] + " # " + inputBuffer
						}
					} else if mode == "prompt_comment_before" {
						if strings.TrimSpace(inputBuffer) != "" {
							lines[promptRealIdx] = "# " + inputBuffer
						} else {
							// Remove the empty line we added
							lines = append(lines[:promptRealIdx], lines[promptRealIdx+1:]...)
						}
					} else if mode == "prompt_replace" {
						lines[promptRealIdx] = inputBuffer
					}
					mode = "normal"
				default:
					if buf[0] >= 32 && buf[0] <= 126 { // Printable ASCII
						if mode == "filter" {
							filterQuery += string(buf[0])
						} else {
							inputBuffer += string(buf[0])
						}
					}
				}
			}
		}

		render()
	}
	return nil
}

func writeConfFile(path string, lines []string) error {
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			_ = file.Close()
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
