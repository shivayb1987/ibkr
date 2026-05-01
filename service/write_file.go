package service

import (
	"fmt"
	"log"
	"os"
)

type FileWriter struct {
}

func NewFileWriter() FileWriter {
	return FileWriter{}
}

func (s FileWriter) Write(fileName, content string) {
	f, err := os.Create(fmt.Sprintf("./../stocks/%s.txt", fileName))

	if err != nil {
		log.Fatal(err)
	}
	//defer f.Close()

	_, err2 := f.WriteString(content)

	if err2 != nil {
		log.Fatal(err2)
	}
}

func (s FileWriter) WriteBytes(fileName string, content []byte) {
	f, err := os.Create(fmt.Sprintf("./../stocks/%s.json", fileName))

	if err != nil {
		log.Fatal(err)
	}
	//defer f.Close()

	_, err2 := f.Write(content)

	if err2 != nil {
		log.Fatal(err2)
	}
}

func (s FileWriter) Append(fileName, content string) {
	path := fmt.Sprintf("./../stocks/%s.txt", fileName)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		f, err = os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
	}
	//defer f.Close()

	_, err2 := f.WriteString(content)

	if err2 != nil {
		log.Fatal(err2)
	}
}
