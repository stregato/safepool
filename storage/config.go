package storage

// type Config struct {
// 	SFTP  *SFTPConfig  `json:"sftp,omitempty" yaml:"sftp,omitempty"`
// 	S3    *S3Config    `json:"s3,omitempty" yaml:"s3,omitempty"`
// 	Local *LocalConfig `json:"local,omitempty" yaml:"local,omitempty"`
// }

// func ParseUrls(urls []string) (Config, error) {
// 	var c Config
// 	for _, url := range urls {
// 		switch {
// 		case strings.HasPrefix(url, "sftp://"):
// 			if s, err := ParseSFTPUrl(url); err == nil {
// 				c.SFTP = &s
// 			} else {
// 				return Config{}, err
// 			}
// 		case strings.HasPrefix(url, "s3://"):
// 			if s, err := ParseS3Url(url); err == nil {
// 				c.S3 = &s
// 			} else {
// 				return Config{}, err
// 			}
// 		}

// 	}
// 	return c, nil
// }

// func ReadConfig(name string) (Config, error) {
// 	var c Config
// 	data, err := os.ReadFile(name)
// 	if err != nil {
// 		return c, err
// 	}
// 	err = yaml.Unmarshal(data, &c)
// 	return c, err
// }

var SampleConfig = []string{
	"sftp://username:password@hostname?key=key",
	"s3://accessKey:secret@s3.eu-central-1.amazonaws.com/bucket",
	"file://path",
}
