package migrations

import (
	"errors"
	"io"
	"io/fs"
	"sort"
	"time"
)

// historyFS combines immutable flat asset directories without copying SQL files.
// Duplicate filenames fail closed instead of depending on provider precedence.
type historyFS struct{ sources []fs.FS }

func (h historyFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name != "." {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	entries := []fs.DirEntry{}
	seen := map[string]bool{}
	for _, source := range h.sources {
		items, err := fs.ReadDir(source, ".")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if item.IsDir() || seen[item.Name()] {
				return nil, &fs.PathError{Op: "readdir", Path: item.Name(), Err: fs.ErrExist}
			}
			seen[item.Name()] = true
			entries = append(entries, item)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (h historyFS) Open(name string) (fs.File, error) {
	entries, err := h.ReadDir(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return &historyDir{entries: entries}, nil
	}
	if fs.ValidPath(name) {
		for _, source := range h.sources {
			file, err := source.Open(name)
			if err == nil {
				return file, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (h historyFS) ReadFile(name string) ([]byte, error) {
	file, err := h.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

type historyDir struct {
	entries []fs.DirEntry
	offset  int
}

func (d *historyDir) Stat() (fs.FileInfo, error) { return historyDirInfo{}, nil }
func (d *historyDir) Close() error               { return nil }
func (d *historyDir) Read([]byte) (int, error)   { return 0, fs.ErrInvalid }
func (d *historyDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n > 0 && d.offset == len(d.entries) {
		return nil, io.EOF
	}
	end := len(d.entries)
	if n > 0 && d.offset+n < end {
		end = d.offset + n
	}
	result := d.entries[d.offset:end]
	d.offset = end
	return result, nil
}

type historyDirInfo struct{}

func (historyDirInfo) Name() string       { return "." }
func (historyDirInfo) Size() int64        { return 0 }
func (historyDirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0555 }
func (historyDirInfo) ModTime() time.Time { return time.Time{} }
func (historyDirInfo) IsDir() bool        { return true }
func (historyDirInfo) Sys() any           { return nil }
