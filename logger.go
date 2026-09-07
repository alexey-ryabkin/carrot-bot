package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

type rotateWriter struct {
	f    *os.File
	name string
	max  int64
}

// При работе с log ручная обработка многопоточности не требуется
func (w *rotateWriter) Write(p []byte) (int, error) {
	if s, _ := w.f.Stat(); s.Size()+int64(len(p)) > w.max {
		w.f.Close()
		os.Rename(w.name, w.name+".old")

		var err error
		w.f, err = os.OpenFile(w.name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return 0, err
		}
	}

	return w.f.Write(p)
}

func InitLogger(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	w := &rotateWriter{
		f:    f,
		name: path,
		max:  10 * 1024 * 1024,
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(io.MultiWriter(w, os.Stdout))

	return func() {
		f.Close()
	}, nil
}
