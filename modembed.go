package modembed

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"time"
)

// ModTimeFS 是一个包装了 embed.FS 的文件系统
// 它为所有文件使用用户提供的固定 ModTime
type ModTimeFS struct {
	embed.FS
	modTime time.Time // 用户设定的统一修改时间
}

// NewModTimeFS 创建一个新的 ModTimeFS 实例
// efs 是底层的 embed.FS
// fixedModTime 是用户希望应用到所有嵌入文件的修改时间
// 如果 fixedModTime 为零值 time.Time{} 则 ModTime 将保持 embed.FS 的默认行为 (即零时)
// 这可能导致 http.FileServer 无法正确处理304
// 建议用户总是提供一个有意义的非零时间
func NewModTimeFS(efs embed.FS, fixedModTime time.Time) *ModTimeFS {
	// 可以选择在这里加一个警告 如果 fixedModTime 是零值
	// if fixedModTime.IsZero() {
	//	 fmt.Fprintln(os.Stderr, "Warning: modembed.NewModTimeFS called with zero time. HTTP 304 caching might not work as expected.")
	// }
	return &ModTimeFS{
		FS:      efs,
		modTime: fixedModTime.UTC(), // 确保使用UTC以保持一致性
	}
}

// --- fs.FileInfo 包装  ---
type modTimeFileInfo struct {
	fs.FileInfo
	modTime time.Time
}

func (mfi *modTimeFileInfo) ModTime() time.Time { return mfi.modTime }
func (mfi *modTimeFileInfo) Name() string       { return mfi.FileInfo.Name() }
func (mfi *modTimeFileInfo) Size() int64        { return mfi.FileInfo.Size() }
func (mfi *modTimeFileInfo) Mode() fs.FileMode  { return mfi.FileInfo.Mode() }
func (mfi *modTimeFileInfo) IsDir() bool        { return mfi.FileInfo.IsDir() }
func (mfi *modTimeFileInfo) Sys() interface{}   { return mfi.FileInfo.Sys() }

// --- fs.DirEntry 包装  ---
type modTimeDirEntry struct {
	fs.DirEntry
	modTime time.Time
}

func (mde *modTimeDirEntry) Info() (fs.FileInfo, error) {
	info, err := mde.DirEntry.Info()
	if err != nil {
		return nil, err
	}
	return &modTimeFileInfo{FileInfo: info, modTime: mde.modTime}, nil
}
func (mde *modTimeDirEntry) Name() string      { return mde.DirEntry.Name() }
func (mde *modTimeDirEntry) IsDir() bool       { return mde.DirEntry.IsDir() }
func (mde *modTimeDirEntry) Type() fs.FileMode { return mde.DirEntry.Type() }

// --- fs.File 包装  ---
type modTimeFile struct {
	fs.File
	modTime time.Time
}

func (mf *modTimeFile) Stat() (fs.FileInfo, error) {
	info, err := mf.File.Stat()
	if err != nil {
		return nil, err
	}
	return &modTimeFileInfo{FileInfo: info, modTime: mf.modTime}, nil
}
func (mf *modTimeFile) Read(p []byte) (int, error) { return mf.File.Read(p) }
func (mf *modTimeFile) Close() error               { return mf.File.Close() }
func (mf *modTimeFile) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := mf.File.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, fmt.Errorf("file does not support Seek")
}
func (mf *modTimeFile) ReadDir(count int) ([]fs.DirEntry, error) {
	if rdf, ok := mf.File.(fs.ReadDirFile); ok {
		entries, err := rdf.ReadDir(count)
		if err != nil {
			return nil, err
		}
		wrappedEntries := make([]fs.DirEntry, len(entries))
		for i, entry := range entries {
			wrappedEntries[i] = &modTimeDirEntry{DirEntry: entry, modTime: mf.modTime}
		}
		return wrappedEntries, nil
	}
	return nil, fmt.Errorf("file is not a directory or does not support ReadDir")
}

// --- ModTimeFS 方法实现  ---
func (mfs *ModTimeFS) Open(name string) (fs.File, error) {
	file, err := mfs.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &modTimeFile{File: file, modTime: mfs.modTime}, nil
}

func (mfs *ModTimeFS) ReadFile(name string) ([]byte, error) {
	return mfs.FS.ReadFile(name) // ModTime不影响内容读取
}

func (mfs *ModTimeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := mfs.FS.ReadDir(name)
	if err != nil {
		return nil, err
	}
	wrappedEntries := make([]fs.DirEntry, len(entries))
	for i, entry := range entries {
		wrappedEntries[i] = &modTimeDirEntry{DirEntry: entry, modTime: mfs.modTime}
	}
	return wrappedEntries, nil
}

// 确保 ModTimeFS 实现了必要的接口
var _ fs.FS = (*ModTimeFS)(nil)
var _ fs.ReadDirFS = (*ModTimeFS)(nil)
var _ fs.ReadFileFS = (*ModTimeFS)(nil)
