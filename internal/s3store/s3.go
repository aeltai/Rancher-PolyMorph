package s3store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aeltai/rancher-polymorph/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	s3     *s3.Client
	bucket string
	prefix string
}

type Options struct {
	Region   string
	Bucket   string
	Prefix   string
	Profile  string
	Endpoint string
}

func NewFromConfig(cfg config.S3) (*Client, error) {
	return New(Options{
		Region:   cfg.Region,
		Bucket:   cfg.Bucket,
		Prefix:   cfg.Prefix,
		Profile:  cfg.Profile,
		Endpoint: cfg.Endpoint,
	})
}

func New(opts Options) (*Client, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required (config s3.bucket or --bucket)")
	}
	loadOpts := []func(*awsconfig.LoadOptions) error{}
	if opts.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		loadOpts = append(loadOpts, awsconfig.WithSharedConfigProfile(opts.Profile))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if opts.Endpoint != "" {
			o.BaseEndpoint = aws.String(opts.Endpoint)
			o.UsePathStyle = true
		}
	})
	return &Client{s3: client, bucket: opts.Bucket, prefix: strings.Trim(opts.Prefix, "/")}, nil
}

func ParseURI(uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("expected s3://bucket/key, got %q", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 || parts[1] == "" {
		return "", "", fmt.Errorf("expected s3://bucket/key, got %q", uri)
	}
	return parts[0], parts[1], nil
}

func (c *Client) fullKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	if c.prefix == "" {
		return key
	}
	return c.prefix + "/" + key
}

func (c *Client) Download(ctx context.Context, uri, destPath string) error {
	bucket, key, err := c.resolveURI(uri)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	resp, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 get s3://%s/%s: %w", bucket, key, err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func (c *Client) resolveURI(uri string) (bucket, key string, err error) {
	if strings.HasPrefix(uri, "s3://") {
		return ParseURI(uri)
	}
	return c.bucket, c.fullKey(uri), nil
}

func (c *Client) Upload(ctx context.Context, localPath, uri string) error {
	var bucket, key string
	var err error
	if strings.HasPrefix(uri, "s3://") {
		bucket, key, err = ParseURI(uri)
		if err != nil {
			return err
		}
	} else {
		bucket = c.bucket
		key = c.fullKey(uri)
	}
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("s3 put s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

func (c *Client) ListKeys(ctx context.Context, prefix string) ([]string, error) {
	p := c.fullKey(strings.TrimPrefix(prefix, "/"))
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(c.s3, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(p),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}
