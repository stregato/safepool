package transport

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/code-to-go/safepool/core"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/sirupsen/logrus"
)

type S3Config struct {
	Region     string `json:"region" yaml:"region"`
	Endpoint   string `json:"endpoint" yaml:"endpoint"`
	Bucket     string `json:"bucket" yaml:"bucket"`
	AccessKey  string `json:"accessKey" yaml:"accessKey"`
	Secret     string `json:"secret" yaml:"secret"`
	DisableSSL bool   `json:"disableSSL" yaml:"disableSSL"`
}

type S3 struct {
	uploader *s3manager.Uploader
	svc      *s3.S3
	bucket   string
	url      string
	touch    map[string]time.Time
}

func ParseS3Url(s string) (S3Config, error) {
	u, err := url.Parse(s)
	if err != nil {
		return S3Config{}, err
	}

	password, _ := u.User.Password()
	return S3Config{
		Bucket:    u.Path,
		AccessKey: u.User.Username(),
		Secret:    password,
	}, nil
}

func S3ToUrl(c S3Config) string {
	return fmt.Sprintf("s3://%s@%s/%s#region-%s", c.AccessKey, c.Endpoint, c.Bucket, c.Region)
}

func getAWSConfig(u *url.URL) *aws.Config {

	accessKey := u.User.Username()
	secret, hasSecret := u.User.Password()
	params := u.Query()

	s3c := aws.Config{}
	if params.Has("region") {
		s3c.Region = aws.String(params.Get("region"))
	}

	if accessKey != "" && hasSecret {
		s3c.Credentials = credentials.NewStaticCredentials(
			accessKey,
			secret,
			"",
		)
	}
	if u.Host != "" {
		s3c.Endpoint = aws.String(u.Host)
	}
	if params.Has("disableSSL") {
		s3c.DisableSSL = aws.Bool(true)
	}

	return &s3c
}

func NewS3(connectionUrl string) (Exchanger, error) {
	u, err := url.Parse(connectionUrl)
	if core.IsErr(err, "invalid url '%s': %v", connectionUrl) {
		return nil, err
	}

	repr := fmt.Sprintf("s3://%s@%s/%s", u.User, u.Host, u.Path)
	sess, err := session.NewSession(getAWSConfig(u))
	if core.IsErr(err, "cannot create S3 session for %s:%v", repr) {
		return nil, err
	}

	s := &S3{
		uploader: s3manager.NewUploader(sess),
		svc:      s3.New(sess),
		url:      repr,
		bucket:   u.Path,
		touch:    map[string]time.Time{},
	}
	err = s.createBucketIfNeeded()
	return s, err
}

func (s *S3) createBucketIfNeeded() error {
	_, err := s.svc.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return err
	}

	_, err = s.svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		logrus.Errorf("cannot create bucket %s: %v", s.bucket, err)
	}

	return err
}

func (s *S3) Touched(name string) bool {
	touchFile := fmt.Sprintf("%s.touch", name)
	h, err := s.svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(touchFile),
	})
	if err != nil {
		return true
	}
	if h.LastModified.After(s.touch[name]) {
		if !core.IsErr(s.Write(touchFile, &bytes.Buffer{}), "cannot write touch file: %v") {
			s.touch[name] = *h.LastModified
		}
		return true
	}
	return false
}

func (s *S3) Read(name string, rang *Range, dest io.Writer) error {
	var r *string
	if rang != nil {
		r = aws.String(fmt.Sprintf("byte%d-%d", rang.From, rang.To))
	}

	rawObject, err := s.svc.GetObject(
		&s3.GetObjectInput{
			Bucket: &s.bucket,
			Key:    &name,
			Range:  r,
		})
	if err != nil {
		logrus.Errorf("cannot read %s/%s: %v", s, name, err)
		return err
	}

	// b, err := io.ReadAll(rawObject.Body)
	// dest.Write(b)
	io.Copy(dest, rawObject.Body)
	// print(n)
	rawObject.Body.Close()
	return nil
}

func (s *S3) Write(name string, source io.Reader) error {

	_, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: &s.bucket,
		Key:    &name,
		Body:   source,
	})
	if err != nil {
		logrus.Errorf("cannot write %s/%s: %v", s.String(), name, err)
	}
	return err
}

func (s *S3) ReadDir(dir string, opts ListOption) ([]fs.FileInfo, error) {
	input := &s3.ListObjectsInput{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(dir + "/"),
		Delimiter: aws.String("/"),
	}

	result, err := s.svc.ListObjects(input)
	if err != nil {
		logrus.Errorf("cannot list %s/%s: %v", s.String(), dir, err)
		return nil, err
	}

	var infos []fs.FileInfo
	for _, item := range result.Contents {
		cut := len(path.Clean(dir))
		name := (*item.Key)[cut+1:]

		infos = append(infos, simpleFileInfo{
			name:    name,
			size:    *item.Size,
			isDir:   false,
			modTime: *item.LastModified,
		})

	}

	return infos, nil
}

func (s *S3) Stat(name string) (fs.FileInfo, error) {
	feed, err := s.svc.HeadObject(&s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &name,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound": // s3.ErrCodeNoSuchKey does not work, aws is missing this error code so we hardwire a string
				return nil, fs.ErrNotExist
			default:
				return nil, fs.ErrInvalid
			}
		}
		return nil, err
	}

	return simpleFileInfo{
		name:    path.Base(name),
		size:    *feed.ContentLength,
		isDir:   strings.HasSuffix(name, "/"),
		modTime: *feed.LastModified,
	}, nil
}

func (s *S3) Rename(old, new string) error {
	_, err := s.svc.CopyObject(&s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(url.QueryEscape(old)),
		Key:        aws.String(new),
	})
	return err
}

func (s *S3) Delete(name string) error {
	input := &s3.ListObjectsInput{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(name + "/"),
		Delimiter: aws.String("/"),
	}

	result, err := s.svc.ListObjects(input)
	if err == nil && len(result.Contents) > 0 {
		for _, item := range result.Contents {
			_, err = s.svc.DeleteObject(&s3.DeleteObjectInput{
				Bucket: &s.bucket,
				Key:    item.Key,
			})
			if core.IsErr(err, "cannot delete %s: %v", item.Key) {
				return err
			}
		}
	} else {
		_, err = s.svc.DeleteObject(&s3.DeleteObjectInput{
			Bucket: &s.bucket,
			Key:    &name,
		})
	}

	core.IsErr(err, "cannot delete %s: %v", name)
	return err
}

func (s *S3) Close() error {
	return nil
}

func (s *S3) String() string {
	return s.url
}
