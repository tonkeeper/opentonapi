package filestorage

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	WalletBackupBucket = "wallet-backup"
)

type FileStorage struct {
	Client *s3.S3
}

func NewFileStorageConnection(accessKeyID, secretAccessKey string) *FileStorage {
	if accessKeyID == "" || secretAccessKey == "" {
		return nil
	}
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(
			accessKeyID,
			secretAccessKey,
			""),
		Endpoint: aws.String("https://ams3.digitaloceanspaces.com"),
		Region:   aws.String("us-east-1"),
	}
	newSession, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatalf("failed to create session file_storage: %v", err)
	}
	s3Client := s3.New(newSession)

	_, err = s3Client.ListBuckets(nil) // ping for success connect
	if err != nil {
		log.Fatalf("failed to connect to file_storage: %v", err)
	}

	return &FileStorage{
		Client: s3Client,
	}
}

func (s *FileStorage) SaveWalletBackup(bucketName, fileName string, data []byte) error {
	_, err := s.Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
		Body:   bytes.NewReader(data),
		ACL:    aws.String("public-read"),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %v", err)
	}

	return nil
}

func (s *FileStorage) GetWalletBackup(bucketName, fileName string) ([]byte, error) {
	resp, err := s.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get file from S3: %v", err)
	}
	defer resp.Body.Close()

	fileData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %v", err)
	}

	return fileData, nil
}
