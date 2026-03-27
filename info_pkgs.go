package g2

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

type InfoPkg struct {
	PackageAtom string
}

func parseInfoPkgsReader(r io.Reader) ([]InfoPkg, error) {
	var results []InfoPkg
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		results = append(results, InfoPkg{
			PackageAtom: line,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading info_pkgs: %w", err)
	}

	return results, nil
}

func ParseInfoPkgsFS(sysFS fs.FS, path string) ([]InfoPkg, error) {
	f, err := sysFS.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseInfoPkgsReader(f)
}

func ParseInfoPkgs(path string) ([]InfoPkg, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return parseInfoPkgsReader(f)
}

func SerializeInfoPkgs(w io.Writer, pkgs []InfoPkg) error {
	for i, pkg := range pkgs {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s", pkg.PackageAtom); err != nil {
			return err
		}
	}
	return nil
}
