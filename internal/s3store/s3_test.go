package s3store

import (
	"strings"
	"testing"
)

func TestNewRequiresBucket(t *testing.T) {
	_, err := New(Options{Region: "eu-central-1"})
	if err == nil {
		t.Fatal("expected error without bucket")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseURI(t *testing.T) {
	bucket, key, err := ParseURI("s3://my-bucket/migrations/backup.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if bucket != "my-bucket" || key != "migrations/backup.tar.gz" {
		t.Fatalf("bucket=%q key=%q", bucket, key)
	}

	_, _, err = ParseURI("https://example.com/x")
	if err == nil {
		t.Fatal("expected error for non-s3 uri")
	}

	_, _, err = ParseURI("s3://bucket-only")
	if err == nil {
		t.Fatal("expected error for bucket-only uri")
	}
}

func TestFullKeyViaClient(t *testing.T) {
	c := &Client{bucket: "b", prefix: "migrations"}
	if got := c.fullKey("backup.tar.gz"); got != "migrations/backup.tar.gz" {
		t.Fatalf("fullKey=%q", got)
	}
	if got := c.fullKey("/nested/key.tgz"); got != "migrations/nested/key.tgz" {
		t.Fatalf("fullKey=%q", got)
	}
	c2 := &Client{bucket: "b", prefix: ""}
	if got := c2.fullKey("backup.tar.gz"); got != "backup.tar.gz" {
		t.Fatalf("fullKey=%q", got)
	}
}
