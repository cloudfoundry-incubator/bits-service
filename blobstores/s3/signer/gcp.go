package signer

import (
	"crypto/hmac"
	"crypto/sha1"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/pkg/errors"
)

type Google struct {
	AccessID        string
	SecretAccessKey string
}

func (g *Google) Sign(req *request.Request, bucket, path string, expires time.Time) (string, error) {
	signedURL, e := storage.SignedURL(bucket, path, &storage.SignedURLOptions{
		GoogleAccessID: g.AccessID,
		SignBytes: func(b []byte) ([]byte, error) {
			hash := hmac.New(sha1.New, []byte(g.SecretAccessKey))
			hash.Write(b)
			return hash.Sum(nil), nil
		},
		Method:  strings.ToUpper(req.HTTPRequest.Method),
		Expires: expires,
	})
	if e != nil {
		return "", errors.Wrapf(e, "Bucket/Path %v/%v", bucket, path)
	}
	return signedURL, nil
}
