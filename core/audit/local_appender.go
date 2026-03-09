package audit

type LocalAppender struct {
	filePath string
}

func NewLocalAppender(path string) *LocalAppender {
	return &LocalAppender{filePath: path}
}
