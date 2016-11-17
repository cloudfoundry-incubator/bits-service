package pathsigner

import (
	"crypto/md5"
	"fmt"
	"net/url"
)

type PathSigner struct {
	Secret string
}

func (signer *PathSigner) Sign(path string) string {
	return fmt.Sprintf("%s?md5=%x", path, md5.Sum([]byte(path+signer.Secret)))
}

func (signer *PathSigner) SignatureValid(u *url.URL) bool {
	return u.Query().Get("md5") == fmt.Sprintf("%x", md5.Sum([]byte(u.Path+signer.Secret)))
} 
