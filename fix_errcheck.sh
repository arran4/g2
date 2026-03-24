sed -i 's/fmt.Fprintln(w, "NAME\\tDESCRIPTION")/\_, \_ = fmt.Fprintln(w, "NAME\\tDESCRIPTION")/g' cmd/g2/ebuild.go
sed -i 's/fmt.Fprintf(w, "%s\\t%s\\n", t.Name, t.Description)/\_, \_ = fmt.Fprintf(w, "%s\\t%s\\n", t.Name, t.Description)/g' cmd/g2/ebuild.go
sed -i 's/w.Flush()/\_ = w.Flush()/g' cmd/g2/ebuild.go
sed -i 's/defer os.RemoveAll(tmpDir)/defer func() { \_ = os.RemoveAll(tmpDir) }()/g' cmd/g2/main_test.go
sed -i 's/cleanup = func() { os.RemoveAll(tmpDir) }/cleanup = func() { \_ = os.RemoveAll(tmpDir) }/g' cmd/g2/site.go
sed -i 's/defer f.Close()/defer func() { \_ = f.Close() }()/g' cmd/g2/site.go
sed -i 's/defer resp.Body.Close()/defer func() { \_ = resp.Body.Close() }()/g' cmd/g2/site.go
sed -i 's/defer os.RemoveAll(tmpDir)/defer func() { \_ = os.RemoveAll(tmpDir) }()/g' manifest_test.go
sed -i 's/defer f.Close()/defer func() { \_ = f.Close() }()/g' metadata.go
