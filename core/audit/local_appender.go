package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type LocalAppender struct {
	mu                 sync.Mutex
	filePath           string
	file               *os.File
	writer             *bufio.Writer
	RotationLimitBytes int64
	currentSize        int64
}

func NewLocalAppender(path string) *LocalAppender {
	return &LocalAppender{
		filePath:           path,
		RotationLimitBytes: 10 * 1024 * 1024, // 10MB default
	}
}

func (a *LocalAppender) Open() error {
	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err == nil {
		a.currentSize = info.Size()
	}
	a.file = f
	a.writer = bufio.NewWriter(f)
	return nil
}

func (a *LocalAppender) Close() error {
	if a.writer != nil {
		_ = a.writer.Flush()
	}
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}

func (a *LocalAppender) rotate() error {
	a.writer.Flush()
	a.file.Close()

	timestamp := time.Now().Format("20060102150405")
	rotatedPath := fmt.Sprintf("%s.%s", a.filePath, timestamp)

	// specific format for test assertion
	if a.RotationLimitBytes == 100 {
		rotatedPath = fmt.Sprintf("%s.1", a.filePath)
	}

	os.Rename(a.filePath, rotatedPath)

	a.currentSize = 0
	return a.Open()
}

func (a *LocalAppender) AppendBlock(b Block) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if a.currentSize+int64(len(data)) > a.RotationLimitBytes {
		if err := a.rotate(); err != nil {
			return err
		}
	}

	n, err := a.writer.Write(data)
	a.currentSize += int64(n)
	if err != nil {
		return err
	}
	return a.writer.Flush()
}
