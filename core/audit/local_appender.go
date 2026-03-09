package audit

import "os"

type LocalAppender struct {
	filePath string
	file     *os.File
}

func NewLocalAppender(path string) *LocalAppender {
	return &LocalAppender{filePath: path}
}

func (a *LocalAppender) Open() error {
	f, err := os.OpenFile(a.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	a.file = f
	return nil
}

func (a *LocalAppender) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
