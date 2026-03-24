sed -i 's/return fmt.Errorf("downloading and calculating checksums: %v\\n", err)/return fmt.Errorf("downloading and calculating checksums: %v", err)/g' cmd/g2/main.go
sed -i 's/return fmt.Errorf("updating manifest: %v\\n", err)/return fmt.Errorf("updating manifest: %v", err)/g' cmd/g2/main.go
sed -i 's/sb.WriteString(fmt.Sprintf("DIST %s %d", filename, checksums.Size))/fmt.Fprintf(\&sb, "DIST %s %d", filename, checksums.Size)/g' cmd/g2/main.go
sed -i 's/sb.WriteString(fmt.Sprintf(" %s %s", name, value))/fmt.Fprintf(\&sb, " %s %s", name, value)/g' cmd/g2/main.go
sed -i 's/durationInSeconds/durationSecs/g' httputil.go
