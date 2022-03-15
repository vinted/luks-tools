package s3store

import (
	"bytes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	log "github.com/sirupsen/logrus"
	"github.com/vinted/luks-tools/pkg/kdumpcollector/config"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FileInfo struct {
	Name    string
	Bucket  string
	SinkSSH bool
	SinkSCP bool
}

type awsS3 struct {
	svc s3iface.S3API
}

func getFileInfo(payload []byte, fileSize int64) FileInfo {
	var fileinfo FileInfo
	cfg := config.Config
	r := regexp.MustCompile(`\/.*`)
	path := r.FindString(string(payload))
	path = strings.Replace(path, string(byte(0)), "", -1)
	// remove backspace characters
	path = strings.Replace(path, string(byte(8)), "", -1)
	fileinfo.Name = strings.Replace(filepath.Base(path), "-incomplete", "", -1)
	fileinfo.Bucket = strings.Replace(filepath.Dir(path), cfg.StripFromBucketName, "", -1)
	fileinfo.Bucket = strings.Replace(fileinfo.Bucket, "/", "", -1)
	fileinfo.Bucket = strings.Replace(fileinfo.Bucket, ".", "-", -1)
	fileinfo.Bucket = strings.Replace(fileinfo.Bucket, ":", "-", -1)
	fileinfo.Name = strings.Replace(filepath.Base(fileinfo.Name), "/", "", -1)
	if len(fileinfo.Bucket) == 0 || fileinfo.Bucket == "-" {
		fileinfo.Bucket = strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	if len(fileinfo.Name) == 0 {
		fileinfo.Name = strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	fileinfo.SinkSSH = fileSize == -1
	fileinfo.SinkSCP = fileSize != -1
	return fileinfo
}

func Upload(channel io.Reader, payload []byte, fileSize int64) error {
	cli, err := s3Client()
	if err != nil {
		return err
	}
	s3Client := awsS3{svc: cli}
	err = s3Client.uploader(channel, payload, fileSize)
	if err != nil {
		return err
	}
	return nil
}

func (s3Client awsS3) uploader(channel io.Reader, payload []byte, fileSize int64) error {
	cfg := config.Config
	s3ChunkSize := cfg.S3ChunkSize
	bufSize := cfg.ReadBuffSize
	chunkCount := 0
	fileInfo := getFileInfo(payload, fileSize)
	var currentSize int64 = 0
	var streamFinished bool = false
	var dataChunkSize int64 = 0
	var completedParts []*s3.CompletedPart
	var multipartUploadOutput *s3.CreateMultipartUploadOutput
	for {
		var content []byte
		for dataChunkSize < s3ChunkSize {
			buf := make([]byte, bufSize)
			if fileInfo.SinkSCP {
				// this concerns scp sink chanel only
				bytesLeft := fileSize - currentSize
				if bytesLeft < int64(bufSize) {
					log.Info("Shrinking buffer to ", bytesLeft)
					// scp does not close channel upon completing - it must read exact number of bytes
					// that was anounced before starting data transfer. thus we need to read precise number
					// of bytes with last read.
					bufSize = int(bytesLeft)
					buf = make([]byte, bufSize)
				}
			}
			bytesRead, err := channel.Read(buf)
			if err == io.EOF {
				streamFinished = true
				log.Info("EOF reached")
				break
			}
			if fileInfo.SinkSSH && bytesRead < bufSize {
				// this concerns ssh sink channel only.
				// there may be bytes less than buffSize left on channel.
				// We need to remove trailing zeros from buffer if this happens.
				buf = buf[:bytesRead]
			}
			content = append(content, buf...)
			dataChunkSize += int64(len(buf))
			currentSize += int64(len(buf))
			if fileSize == currentSize {
				streamFinished = true
				log.Info("Last Byte reached")
				break
			}
		}
		if streamFinished {
			// final chunk
			if chunkCount == 0 {
				// single chunk, stream finished
				log.Info("Total data size is: ", currentSize, " making singlepart upload")
				err := s3Client.createBucket(fileInfo.Bucket)
				if err != nil {
					return err
				}
				err = s3Client.simpleUpload(fileInfo.Bucket, fileInfo.Name, content)
				if err != nil {
					return err
				}
			} else {
				// upload last chunk. Finish multipart
				log.Info("Last chunk size is: ", dataChunkSize, " Finishing multipart upload")
				completedPart, err := s3Client.uploadPart(multipartUploadOutput, chunkCount+1, content)
				if err != nil {
					er := s3Client.abortMultipartUpload(multipartUploadOutput)
					if er != nil {
						log.Error("Multipart abort failed. ", er)
					}
					return err
				}
				completedParts = append(completedParts, completedPart)
				err = s3Client.finishMultipartUpload(multipartUploadOutput, completedParts)
				if err != nil {
					er := s3Client.abortMultipartUpload(multipartUploadOutput)
					if er != nil {
						log.Error("Multipart abort failed. ", er)
					}
					return err
				}
			}
			// finish loop.
			break
		} else {
			// this is a fully-filled chunk. Upload it and clear buffer
			if chunkCount == 0 {
				// this is a first chunk of a multipart upload. Initiate multipart upload
				err := s3Client.createBucket(fileInfo.Bucket)
				if err != nil {
					return err
				}
				multipartUploadOutput, err = s3Client.createMultipartUpload(fileInfo.Bucket, fileInfo.Name)
				if err != nil {
					return err
				}
			}
			completedPart, err := s3Client.uploadPart(multipartUploadOutput, chunkCount+1, content)
			if err != nil {
				er := s3Client.abortMultipartUpload(multipartUploadOutput)
				if er != nil {
					log.Error("Multipart abort failed. ", er)
				}
				return err
			}
			completedParts = append(completedParts, completedPart)
		}
		chunkCount++
		// chunk was uploaded. Prepare empty chunk
		dataChunkSize = 0
	}
	log.Info("Upload finished. File size: ", currentSize)
	return nil
}

func s3Client() (s3iface.S3API, error) {
	cfg := config.Config
	creds := credentials.NewStaticCredentials(cfg.S3AccessKeyID, cfg.S3SecretKey, "")
	_, err := creds.Get()
	if err != nil {
		log.Error("bad credentials: ", err)
		return nil, err
	}
	awsCfg := aws.NewConfig().WithCredentials(creds).WithEndpoint(cfg.S3Endpoint).WithRegion(cfg.S3Region).WithS3ForcePathStyle(cfg.S3ForcePathStyle)
	sess, err := session.NewSession()
	if err != nil {
		log.Error("Unable to create new S3 session: ", err)
		return nil, err
	}
	svc := s3.New(sess, awsCfg)
	return svc, nil
}

func (s awsS3) createMultipartUpload(bucketName string, objectName string) (*s3.CreateMultipartUploadOutput, error) {
	log.Info("Creating multipart upload")
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}
	result, err := s.svc.CreateMultipartUpload(input)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s awsS3) uploadPart(multipartData *s3.CreateMultipartUploadOutput, part int, body []byte) (*s3.CompletedPart, error) {
	log.Info("Uploading part: ", part, " size: ", len(body))
	input := &s3.UploadPartInput{
		Body:       bytes.NewReader([]byte(body)),
		Bucket:     multipartData.Bucket,
		Key:        multipartData.Key,
		PartNumber: aws.Int64(int64(part)),
		UploadId:   multipartData.UploadId,
	}
	result, err := s.svc.UploadPart(input)
	if err != nil {
		return nil, err
	}
	uploadResult := &s3.CompletedPart{
		ETag:       result.ETag,
		PartNumber: aws.Int64(int64(part)),
	}
	log.Info(uploadResult)
	return uploadResult, nil
}

func (s awsS3) finishMultipartUpload(multipartData *s3.CreateMultipartUploadOutput, completedParts []*s3.CompletedPart) error {
	log.Info("Finishing multipart upload")
	input := &s3.CompleteMultipartUploadInput{
		Bucket:   multipartData.Bucket,
		Key:      multipartData.Key,
		UploadId: multipartData.UploadId,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	_, err := s.svc.CompleteMultipartUpload(input)
	return err
}

func (s awsS3) abortMultipartUpload(multipartData *s3.CreateMultipartUploadOutput) error {
	log.Info("Aborting multipart upload for UploadId: ", multipartData.UploadId)
	input := &s3.AbortMultipartUploadInput{
		Bucket:   multipartData.Bucket,
		Key:      multipartData.Key,
		UploadId: multipartData.UploadId,
	}
	_, err := s.svc.AbortMultipartUpload(input)
	return err
}

func (s awsS3) createBucket(bucketName string) error {
	log.Info("Creating bucket: ", bucketName)
	_, err := s.svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Error("Unable to create bucket: ", err)
		return err
	}
	err = s.svc.WaitUntilBucketExists(&s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		log.Error("Error occurred while waiting for bucket to be created: ", err)
		return err
	}
	return nil
}

func (s *awsS3) simpleUpload(bucketName string, objectName string, body []byte) error {
	// implements simple upload - creates object from single body
	input := &s3.PutObjectInput{
		Body:   bytes.NewReader([]byte(body)),
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectName),
	}
	_, err := s.svc.PutObject(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				log.Error(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			log.Error(err.Error())
		}
		return err
	}
	return nil
}
