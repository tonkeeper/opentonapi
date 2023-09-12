package filestorage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

type LocalFileStorage struct{}

func NewLocalFileStorage() *LocalFileStorage {
	return &LocalFileStorage{}
}

func (s *LocalFileStorage) SaveWalletBackup(bucketName, fileName string, data []byte) error {
	tempFileName := fileName + fmt.Sprintf(".temp%v", time.Now().Nanosecond()+time.Now().Second())
	file, err := os.Create(tempFileName)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := bytes.NewReader(data)
	buffer := make([]byte, len(data))
	if _, err = reader.Read(buffer); err != nil {
		return err
	}
	_, err = io.Copy(file, io.LimitReader(reader, 640*1024)) //640K ought to be enough for anybody
	if err != nil {
		return err
	}
	err = os.Rename(tempFileName, fileName)
	if err != nil {
		return err
	}

	return nil
}

func (s *LocalFileStorage) GetWalletBackup(bucketName, fileName string) ([]byte, error) {
	dump, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	return dump, nil
}
