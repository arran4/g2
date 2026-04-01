package main

import (
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/go-git/go-billy/v5"
)

type BillyFS struct {
	bfs billy.Filesystem
}

func NewBillyFS(bfs billy.Filesystem) *BillyFS {
	return &BillyFS{bfs: bfs}
}

func (b *BillyFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	info, err := b.bfs.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	if info.IsDir() {
		return &billyDirFile{
			fs:   b.bfs,
			name: name,
			info: info,
		}, nil
	}

	f, err := b.bfs.Open(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}

	return &billyFile{
		File: f,
		info: info,
		name: name,
	}, nil
}

func (b *BillyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}

	entries, err := b.bfs.ReadDir(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}

	dirEntries := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		dirEntries[i] = fs.FileInfoToDirEntry(entry)
	}
	return dirEntries, nil
}

func (b *BillyFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}

	info, err := b.bfs.Stat(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return info, nil
}

func (b *BillyFS) ReadFile(name string) ([]byte, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrInvalid}
	}

	f, err := b.bfs.Open(name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &fs.PathError{Op: "readfile", Path: name, Err: fs.ErrNotExist}
		}
		return nil, &fs.PathError{Op: "readfile", Path: name, Err: err}
	}
	defer f.Close()

	return io.ReadAll(f)
}

func (b *BillyFS) Sub(dir string) (fs.FS, error) {
	if dir == "." {
		return b, nil
	}
	ch, err := b.bfs.Chroot(dir)
	if err != nil {
		return nil, err
	}
	return NewBillyFS(ch), nil
}

type billyFile struct {
	billy.File
	info fs.FileInfo
	name string
}

func (f *billyFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

type billyDirFile struct {
	fs     billy.Filesystem
	name   string
	info   fs.FileInfo
	offset int
}

func (d *billyDirFile) Stat() (fs.FileInfo, error) {
	return d.info, nil
}

func (d *billyDirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

func (d *billyDirFile) Close() error {
	return nil
}

func (d *billyDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	infos, err := d.fs.ReadDir(d.name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: d.name, Err: err}
	}

	if d.offset >= len(infos) {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}

	var end int
	if n <= 0 {
		end = len(infos)
	} else {
		end = d.offset + n
		if end > len(infos) {
			end = len(infos)
		}
	}

	entries := make([]fs.DirEntry, end-d.offset)
	for i, info := range infos[d.offset:end] {
		entries[i] = fs.FileInfoToDirEntry(info)
	}

	d.offset = end
	return entries, nil
}

// Ensure billy wrapper functions implement required interfaces
var _ fs.FS = (*BillyFS)(nil)
var _ fs.ReadDirFS = (*BillyFS)(nil)
var _ fs.StatFS = (*BillyFS)(nil)
var _ fs.ReadFileFS = (*BillyFS)(nil)
var _ fs.SubFS = (*BillyFS)(nil)
