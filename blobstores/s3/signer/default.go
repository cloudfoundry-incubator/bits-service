package signer

import (
	"time"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/pkg/errors"
)

type Default struct{}

func (d *Default) Sign(req *request.Request, bucket, path string, expires time.Time) (string, error) {
	signedURL, e := req.Presign(expires.Sub(time.Now()))
	if e != nil {
		return "", errors.Wrapf(e, "Bucket/Path %v/%v", bucket, path)
	}
	return signedURL, nil
}
