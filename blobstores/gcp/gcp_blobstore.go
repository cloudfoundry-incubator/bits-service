package gcp

import (
	"context"
	"io"
	"time"

	"golang.org/x/oauth2/jwt"

	"cloud.google.com/go/storage"

	"github.com/petergtz/bitsgo"
	"github.com/petergtz/bitsgo/blobstores/validate"
	"github.com/petergtz/bitsgo/config"
	"github.com/petergtz/bitsgo/logger"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Blobstore struct {
	client    *storage.Client
	jwtConfig *jwt.Config
	bucket    string
}

func NewBlobstore(config config.GCPBlobstoreConfig) *Blobstore {
	validate.NotEmpty(config.Bucket)
	validate.NotEmpty(config.Email)
	validate.NotEmpty(config.PrivateKey)
	validate.NotEmpty(config.PrivateKeyID)
	validate.NotEmpty(config.TokenURL)

	ctx := context.Background()

	jwtConfig := &jwt.Config{
		Email:        config.Email,
		PrivateKey:   []byte(config.PrivateKey),
		PrivateKeyID: config.PrivateKeyID,
		Scopes:       []string{storage.ScopeFullControl},
		TokenURL:     config.TokenURL,
	}
	client, err := storage.NewClient(ctx, option.WithTokenSource(jwtConfig.TokenSource(ctx)))
	if err != nil {
		panic(err)
	}
	return &Blobstore{
		client:    client,
		bucket:    config.Bucket,
		jwtConfig: jwtConfig,
	}
}

func (blobstore *Blobstore) Exists(path string) (bool, error) {
	_, e := blobstore.client.Bucket(blobstore.bucket).Object(path).NewReader(context.Background())

	if e != nil {
		if e == storage.ErrObjectNotExist || e == storage.ErrBucketNotExist {
			return false, nil
		}
		return false, errors.Wrapf(e, "Failed to check for %v/%v", blobstore.bucket, path)
	}
	return true, nil
}

func (blobstore *Blobstore) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	return storage.SignedURL(blobstore.bucket, path, &storage.SignedURLOptions{
		GoogleAccessID: blobstore.jwtConfig.Email,
		PrivateKey:     blobstore.jwtConfig.PrivateKey,
		Method:         "GET",
		Expires:        time.Now().Add(time.Hour),
	})
}

func (blobstore *Blobstore) Get(path string) (body io.ReadCloser, err error) {
	logger.Log.Debugw("Get from GCP", "bucket", blobstore.bucket, "path", path)
	reader, e := blobstore.client.Bucket(blobstore.bucket).Object(path).NewReader(context.Background())
	if e != nil {
		if e == storage.ErrBucketNotExist || e == storage.ErrObjectNotExist {
			return nil, bitsgo.NewNotFoundError()
		}
		return nil, errors.Wrapf(e, "Path %v", path)
	}
	return reader, nil
}

func (blobstore *Blobstore) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	signedUrl, e := blobstore.HeadOrRedirectAsGet(path)
	return nil, signedUrl, e
}

func (blobstore *Blobstore) Put(path string, src io.ReadSeeker) error {
	logger.Log.Debugw("Put to GCP", "bucket", blobstore.bucket, "path", path)
	if e := blobstore.bucketExists(); e != nil {
		return e
	}
	writer := blobstore.client.Bucket(blobstore.bucket).Object(path).NewWriter(context.TODO())
	_, e := io.Copy(writer, src)
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	e = writer.Close()
	if e != nil {
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) bucketExists() error {
	_, e := blobstore.client.Bucket(blobstore.bucket).Attrs(context.TODO())
	if e != nil {
		if e == storage.ErrBucketNotExist {
			return bitsgo.NewNotFoundErrorWithMessage("bucket '" + blobstore.bucket + "' does not exist")
		}
		return errors.Wrapf(e, "Error while checking for bucket existence. Bucket '%v'", blobstore.bucket)
	}
	return nil
}

func (blobstore *Blobstore) Copy(src, dest string) error {
	logger.Log.Debugw("Copy in GCP", "bucket", blobstore.bucket, "src", src, "dest", dest)
	_, e := blobstore.client.Bucket(blobstore.bucket).Object(dest).CopierFrom(blobstore.client.Bucket(blobstore.bucket).Object(src)).Run(context.Background())
	if e != nil {
		if e == storage.ErrBucketNotExist || e == storage.ErrObjectNotExist {
			return bitsgo.NewNotFoundError()
		}
		return errors.Wrapf(e, "Error while trying to copy src %v to dest %v in bucket %v", src, dest, blobstore.bucket)
	}
	return nil
}

func (blobstore *Blobstore) Delete(path string) error {
	e := blobstore.client.Bucket(blobstore.bucket).Object(path).Delete(context.Background())
	if e != nil {
		if e == storage.ErrBucketNotExist || e == storage.ErrObjectNotExist {
			return bitsgo.NewNotFoundError()
		}
		return errors.Wrapf(e, "Path %v", path)
	}
	return nil
}

func (blobstore *Blobstore) DeleteDir(prefix string) error {
	deletionErrs := []error{}
	it := blobstore.client.Bucket(blobstore.bucket).Objects(context.Background(), &storage.Query{Prefix: prefix})
	for {
		attrs, e := it.Next()
		if e == iterator.Done {
			break
		}
		if e != nil {
			return errors.Wrapf(e, "Prefix %v", prefix)
		}
		e = blobstore.Delete(attrs.Name)
		if e != nil {
			if _, isNotFoundError := e.(*bitsgo.NotFoundError); !isNotFoundError {
				deletionErrs = append(deletionErrs, e)
			}
		}
	}
	if len(deletionErrs) != 0 {
		return errors.Errorf("Prefix %v, errors from deleting: %v", prefix, deletionErrs)
	}
	return nil
}
