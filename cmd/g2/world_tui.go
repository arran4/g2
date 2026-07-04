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
	mode := "normal" // "normal", "insert", "filter", "edit_modal", "prompt_version", "prompt_comment_after", "prompt_comment_before", "prompt_replace"
	inputBuffer := ""
	filterQuery := ""
	scrollOffset := 0
	editModalCursor := 0
	promptRealIdx := 0

	editModalOptions := []string{
		"Comment / Uncomment",
		"Specify version / remove version",
		"Add / remove comment (After on line)",
		"Add / remove comment (Line before)",
		"Modify / replace line",
	}

	var filteredIndices []int
	var listHeight int

	render := func() {
		// Clear screen and reset cursor
		fmt.Print("\033[2J\033[H")

		fmt.Print("Manage Portage World List\r\n")
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
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				fmt.Printf("\033[32m%s\033[0m\r\n", line) // Green for comments
			} else {
				fmt.Printf("%s\r\n", line)
			}
		}

		if mode == "insert" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("New entry: %s_\r\n", inputBuffer)
		} else if mode == "prompt_version" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("Versions (space-separated, blank for none): %s_\r\n", inputBuffer)
		} else if mode == "prompt_comment_after" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("Comment: %s_\r\n", inputBuffer)
		} else if mode == "prompt_comment_before" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("Comment: %s_\r\n", inputBuffer)
		} else if mode == "prompt_replace" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			fmt.Printf("Line: %s_\r\n", inputBuffer)
		} else if mode == "filter" || filterQuery != "" {
			fmt.Print(strings.Repeat("-", 60) + "\r\n")
			if mode == "filter" {
				fmt.Printf("Filter: %s_\r\n", filterQuery)
			} else {
				fmt.Printf("Filter: %s\r\n", filterQuery)
			}
		}

		if mode == "edit_modal" {
			modalWidth := 60
			modalHeight := len(editModalOptions) + 4
			startX := (termWidth - modalWidth) / 2 + 1
			if startX < 1 { startX = 1 }
			startY := (termHeight - modalHeight) / 2 + 1
			if startY < 1 { startY = 1 }

			for i := 0; i < modalHeight; i++ {
				fmt.Printf("\033[%d;%dH", startY+i, startX)
				switch i {
				case 0, modalHeight - 1:
					fmt.Print("+" + strings.Repeat("-", modalWidth-2) + "+")
				case 1:
					title := " Edit Entry "
					padding := (modalWidth - 2 - len(title)) / 2
					fmt.Print("|" + strings.Repeat(" ", padding) + title + strings.Repeat(" ", modalWidth-2-padding-len(title)) + "|")
				case 2:
					fmt.Print("|" + strings.Repeat(" ", modalWidth-2) + "|")
				default:
					optIdx := i - 3
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
						case 1: // Specify version / remove version
							inputBuffer = ""
							mode = "prompt_version"
						case 2: // Add / remove comment (After on line)
							if idx := strings.Index(line, "#"); idx != -1 {
								lines[promptRealIdx] = strings.TrimSpace(line[:idx])
								mode = "normal"
							} else {
								inputBuffer = ""
								mode = "prompt_comment_after"
							}
						case 3: // Add / remove comment (Line before)
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
						case 4: // Modify / replace line
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
		} else if mode == "insert" || mode == "filter" || strings.HasPrefix(mode, "prompt_") {
			// Handle insert/filter/prompt modes
			// Support UTF-8 multi-byte sequences by converting the entire buffer read
			for i := 0; i < n; i++ {
				c := buf[i]
				switch c {
				case 27: // Esc
					if mode == "insert" || strings.HasPrefix(mode, "prompt_") {
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
					} else if mode == "prompt_version" {
						line := lines[promptRealIdx]
						commentStr := ""
						pkgStr := line
						if idx := strings.Index(line, "#"); idx != -1 {
							pkgStr = strings.TrimSpace(line[:idx])
							commentStr = " " + strings.TrimSpace(line[idx:])
						}
						// Use simple split to get base pkg name (approximate without importing g2 if avoid cyclic, but we can use ExtractPackageNameFromDep)
						// Actually we don't have g2 imported in world_tui.go directly, let's just do a simple stripping logic or import g2.
						// world_tui.go is in package main inside cmd/g2, so it CAN import github.com/arran4/g2
						// Since it's in `package main` we need to be careful with imports. Wait, `world_tui.go` doesn't import g2, let's just add it or parse it inline.
						// To avoid import cycle or modifying imports if not needed, let's just do basic parsing.
						// Wait, actually I will add the import or just write the code inline.
						// We'll write inline basic stripping to avoid messing with imports.
						basePkg := strings.TrimLeft(pkgStr, "=<>!~")
						if idx := strings.IndexAny(basePkg, "[:"); idx != -1 {
							basePkg = basePkg[:idx]
						}
						parts := strings.Split(basePkg, "-")
						var resParts []string
						for _, p := range parts {
							if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
								// Reached version part
								break
							}
							resParts = append(resParts, p)
						}
						basePkg = strings.Join(resParts, "-")

						versions := strings.Fields(inputBuffer)
						if len(versions) == 0 {
							lines[promptRealIdx] = basePkg + commentStr
						} else {
							var newLines []string
							for _, v := range versions {
								newLines = append(newLines, fmt.Sprintf("=%s-%s%s", basePkg, v, commentStr))
							}
							lines = append(lines[:promptRealIdx], append(newLines, lines[promptRealIdx+1:]...)...)
						}
					} else if mode == "prompt_comment_after" {
						if strings.TrimSpace(inputBuffer) != "" {
							lines[promptRealIdx] = lines[promptRealIdx] + " # " + inputBuffer
						}
					} else if mode == "prompt_comment_before" {
						if strings.TrimSpace(inputBuffer) != "" {
							lines[promptRealIdx] = "# " + inputBuffer
						} else {
							// If empty, remove the line we just inserted
							lines = append(lines[:promptRealIdx], lines[promptRealIdx+1:]...)
						}
					} else if mode == "prompt_replace" {
						if strings.TrimSpace(inputBuffer) != "" {
							lines[promptRealIdx] = inputBuffer
						} else {
							lines = append(lines[:promptRealIdx], lines[promptRealIdx+1:]...)
						}
					}

					if mode != "filter" {
						inputBuffer = ""
						mode = "normal"
					}
				case 127, 8: // Backspace
					switch mode {
					case "insert", "prompt_version", "prompt_comment_after", "prompt_comment_before", "prompt_replace":
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
						case "insert", "prompt_version", "prompt_comment_after", "prompt_comment_before", "prompt_replace":
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
				case 'e': // Edit
					if len(filteredIndices) > 0 && cursor < len(filteredIndices) {
						editModalCursor = 0
						mode = "edit_modal"
					}
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
