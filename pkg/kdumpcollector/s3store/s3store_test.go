package s3store

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/config"
	"io"
	"os"
	"strings"
	"testing"
)

type s3ClientMock struct {
	s3iface.S3API
}

var bucketName, objectName string
var fileContent []byte
var partCount, partSize int

func (m *s3ClientMock) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	bucketName = aws.StringValue(input.Bucket)
	return nil, nil
}

func (m *s3ClientMock) WaitUntilBucketExists(input *s3.HeadBucketInput) error {
	return nil
}

func (m *s3ClientMock) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	fileContent, _ = io.ReadAll(input.Body)
	objectName = aws.StringValue(input.Key)
	return nil, nil
}

func (m *s3ClientMock) CreateMultipartUpload(input *s3.CreateMultipartUploadInput) (*s3.CreateMultipartUploadOutput, error) {
	objectName = aws.StringValue(input.Key)
	output := &s3.CreateMultipartUploadOutput{
		Bucket:   input.Bucket,
		Key:      input.Key,
		UploadId: aws.String("mockid"),
	}
	return output, nil
}

func (m *s3ClientMock) UploadPart(input *s3.UploadPartInput) (*s3.UploadPartOutput, error) {
	partCount += 1
	part, _ := io.ReadAll(input.Body)
	partSize = len(part)
	output := &s3.UploadPartOutput{
		ETag: aws.String("mocktag"),
	}
	return output, nil
}

func (m *s3ClientMock) CompleteMultipartUpload(input *s3.CompleteMultipartUploadInput) (*s3.CompleteMultipartUploadOutput, error) {
	return nil, nil
}

func setEnv() {
	os.Setenv("KDUMP_S3_ENDPOINT", "http://localhost")
	os.Setenv("KDUMP_S3_ACCESS_KEY_ID", "keyId")
	os.Setenv("KDUMP_S3_SECRET_KEY", "secretKey")
	config.Config = config.ParseConfig()
}

func TestGetFileInfo(t *testing.T) {
	setEnv()
	config.Config = config.ParseConfig()
	payload := "dd of=/var/crash/192.168.57.3-2022-02-17-16:17:54/vmcore-dmesg-incomplete.txt"
	fileOverSsh := getFileInfo([]byte(payload), int64(-1))
	if fileOverSsh.SinkSSH != true {
		t.Fatal("fileOverSsh.SinkSSH should be true, received: ", fileOverSsh.SinkSSH)
	}
	if fileOverSsh.SinkSCP != false {
		t.Fatal("fileOverSsh.SinkSCP should be false, received: ", fileOverSsh.SinkSCP)
	}
	if fileOverSsh.Name != "vmcore-dmesg.txt" {
		t.Fatal("Wrong file name. Received: ", fileOverSsh.Name)
	}
	if fileOverSsh.Bucket != "192-168-57-3-2022-02-17-16-17-54" {
		t.Fatal("Wrong bucket name. Received: ", fileOverSsh.Bucket)
	}
	os.Clearenv()
}

func TestUploader(t *testing.T) {
	setEnv()
	config.Config = config.ParseConfig()
	data := "file data"
	channel := strings.NewReader(data)
	payload := []byte("dd of=/var/crash/NewBucket/vmcore-dmesg-incomplete.txt")
	fileSize := int64(len(data))
	err := awsS3{
		svc: &s3ClientMock{},
	}.uploader(channel, payload, fileSize)
	if err != nil {
		t.Fatal("Got error: ", err)
	}
	if bucketName != "NewBucket" {
		t.Fatal("Wrong bucket name received: ", bucketName)
	}
	if string(fileContent) != "file data" {
		t.Fatal("Wrong file content received: ", string(fileContent))
	}
	if objectName != "vmcore-dmesg.txt" {
		t.Fatal("Wrong file name received: ", objectName)
	}
	os.Clearenv()
}

func TestMultipartUploader(t *testing.T) {
	os.Setenv("KDUMP_S3_CHUNK_SIZE", "10")
	os.Setenv("KDUMP_READ_BUFF_SIZE", "1")
	setEnv()
	data := "123456789012345"
	channel := strings.NewReader(data)
	payload := []byte("dd of=/var/crash/NewBucket/multipart_test.txt")
	fileSize := int64(len(data))
	err := awsS3{
		svc: &s3ClientMock{},
	}.uploader(channel, payload, fileSize)
	if err != nil {
		t.Fatal("Got error: ", err)
	}
	if objectName != "multipart_test.txt" {
		t.Fatal("Wrong file name received: ", objectName)
	}
	if bucketName != "NewBucket" {
		t.Fatal("Wrong bucket name received: ", bucketName)
	}
	if partCount != 2 {
		t.Fatal("Upload should consist of 2 parts, got: ", partCount)
	}
	if partSize != 5 {
		t.Fatal("Last part should consist of 5 bytes, got: ", partSize)
	}
}
