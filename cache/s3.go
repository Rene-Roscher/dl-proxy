package cache

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Client wraps AWS S3 operations
type S3Client struct {
	uploader *s3manager.Uploader
	s3       *s3.S3
	bucket   string
	prefix   string
}

// NewS3Client creates a new S3 client
// For Cloudflare R2, set endpoint to your R2 account endpoint
func NewS3Client(region, accessKey, secretKey, bucket, prefix, endpoint string) (*S3Client, error) {
	// Create AWS config
	awsConfig := &aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKey,
			secretKey,
			"", // token
		),
	}

	// Add custom endpoint if provided (for R2 or other S3-compatible services)
	if endpoint != "" {
		awsConfig.Endpoint = aws.String(endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true) // Required for R2
	}

	// Create AWS session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &S3Client{
		uploader: s3manager.NewUploader(sess),
		s3:       s3.New(sess),
		bucket:   bucket,
		prefix:   prefix,
	}, nil
}

// Upload uploads a file to S3 from a reader
// Returns the S3 path of the uploaded file
func (c *S3Client) Upload(reader io.Reader, cacheKey, filename string, contentType string) (string, error) {
	// Construct S3 key: prefix/cacheKey/filename
	key := fmt.Sprintf("%s/%s/%s", c.prefix, cacheKey, filename)

	input := &s3manager.UploadInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	// Upload to S3
	_, err := c.uploader.Upload(input)
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	return key, nil
}

// GeneratePresignedURL creates a presigned URL for downloading from S3
func (c *S3Client) GeneratePresignedURL(s3Path string, expiry time.Duration) (string, error) {
	req, _ := c.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(s3Path),
	})

	url, err := req.Presign(expiry)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}

// DeleteObject deletes an object from S3
func (c *S3Client) DeleteObject(s3Path string) error {
	_, err := c.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(s3Path),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from S3: %w", err)
	}

	return nil
}
