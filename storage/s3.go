package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/code-to-go/safepool/core"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/smithy-go/logging"
	"github.com/sirupsen/logrus"
)

type S3 struct {
	client *s3.Client
	bucket string
	url    string
}

type s3logger struct{}

func (l s3logger) Logf(classification logging.Classification, format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func OpenS3(connectionUrl string) (Storage, error) {
	u, err := url.Parse(connectionUrl)
	if core.IsErr(err, "invalid url '%s': %v", connectionUrl) {
		return nil, err
	}

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s", u.Host),
		}, nil
	})

	q := u.Query()
	verbose := q.Get("verbose")
	accessKey := q.Get("accessKey")
	secret := q.Get("secret")
	bucket := strings.Trim(u.Path, "/")
	repr := fmt.Sprintf("s3://%s/%s?accessKey=%s", u.Host, bucket, accessKey)

	options := []func(*config.LoadOptions) error{
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secret, "")),
	}
	switch verbose {
	case "1":
		options = append(options,
			config.WithLogger(s3logger{}),
			config.WithClientLogMode(aws.LogRequest|aws.LogResponse),
		)
	case "2":
		options = append(options,
			config.WithLogger(s3logger{}),
			config.WithClientLogMode(aws.LogRequestWithBody|aws.LogResponseWithBody),
		)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	if core.IsErr(err, "cannot create S3 config for %s:%v", repr) {
		return nil, err
	}

	s := &S3{
		client: s3.NewFromConfig(cfg),
		url:    repr,
		bucket: bucket,
	}

	err = s.createBucketIfNeeded()

	return s, s.mapError(err)
}

func (s *S3) createBucketIfNeeded() error {
	_, err := s.client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	_, err = s.client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	core.IsErr(err, "cannot create bucket %s: %v", s.bucket)

	return s.mapError(err)
}

func (s *S3) GetCheckpoint(name string) int64 {
	h, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		return -1
	}
	return h.LastModified.UnixMicro()
}

func (s *S3) SetCheckpoint(name string) (int64, error) {
	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
		Body:   &bytes.Buffer{},
	})
	if core.IsErr(err, "cannot set checkpoint '%s'") {
		return 0, err
	}
	return s.GetCheckpoint(name), nil
}

func (s *S3) Read(name string, rang *Range, dest io.Writer, progress chan int64) error {
	// var r *string
	// if rang != nil {
	// 	r = aws.String(fmt.Sprintf("byte%d-%d", rang.From, rang.To))
	// }

	rawObject, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
		//		Range:  r,
	})
	if err != nil {
		err = s.mapError(err)
		if os.IsNotExist(err) || core.IsErr(err, "cannot read %s/%s: %v", s, name) {
			return err
		}
	}

	_, err = io.Copy(dest, rawObject.Body)
	if core.IsErr(err, "cannot read %s/%s: %v", s, name) {
		return err
	}

	rawObject.Body.Close()
	return nil
}

func (s *S3) Write(name string, source io.ReadSeeker, size int64, progress chan int64) error {
	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &name,
		Body:          source,
		ContentLength: size,
	})
	core.IsErr(err, "cannot write %s/%s: %v", s, name)
	return s.mapError(err)
}

func (s *S3) ReadDir(dir string, opts ListOption) ([]fs.FileInfo, error) {
	input := &s3.ListObjectsV2Input{

		Bucket: aws.String(s.bucket),
		Prefix: aws.String(dir + "/"),
		//		Prefix:    aws.String("ch.safepool/"),
		Delimiter: aws.String("/"),
	}

	result, err := s.client.ListObjectsV2(context.TODO(), input)
	if err != nil {
		logrus.Errorf("cannot list %s/%s: %v", s.String(), dir, err)
		return nil, s.mapError(err)
	}

	var infos []fs.FileInfo

	for _, item := range result.CommonPrefixes {
		cut := len(path.Clean(dir))
		name := strings.TrimRight((*item.Prefix)[cut+1:], "/")

		infos = append(infos, simpleFileInfo{
			name:  name,
			isDir: true,
		})
	}

	for _, item := range result.Contents {
		cut := len(path.Clean(dir))
		name := (*item.Key)[cut+1:]

		infos = append(infos, simpleFileInfo{
			name:    name,
			size:    item.Size,
			isDir:   false,
			modTime: *item.LastModified,
		})

	}

	return infos, nil
}

func (s *S3) mapError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return fs.ErrNotExist
		default:
			return err
		}
	} else {
		return err
	}
}

func (s *S3) Stat(name string) (fs.FileInfo, error) {
	feed, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
	})
	if err != nil {
		return nil, s.mapError(err)
	}

	return simpleFileInfo{
		name:    path.Base(name),
		size:    feed.ContentLength,
		isDir:   strings.HasSuffix(name, "/"),
		modTime: *feed.LastModified,
	}, nil
}

func (s *S3) Rename(old, new string) error {
	_, err := s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(url.QueryEscape(old)),
		Key:        aws.String(new),
	})
	return s.mapError(err)
}

func (s *S3) Delete(name string) error {
	input := &s3.ListObjectsInput{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(name + "/"),
		Delimiter: aws.String("/"),
	}

	result, err := s.client.ListObjects(context.TODO(), input)
	if err == nil && len(result.Contents) > 0 {
		for _, item := range result.Contents {
			_, err = s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: &s.bucket,
				Key:    item.Key,
			})
			if core.IsErr(err, "cannot delete %s: %v", item.Key) {
				return s.mapError(err)
			}
		}
	} else {
		_, err = s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: &s.bucket,
			Key:    &name,
		})
	}

	core.IsErr(err, "cannot delete %s: %v", name)
	return s.mapError(err)
}

func (s *S3) Close() error {
	return nil
}

func (s *S3) String() string {
	return s.url
}
