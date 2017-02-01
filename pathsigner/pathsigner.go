package pathsigner

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/benbjohnson/clock"
)

type PathSigner interface {
	Sign(path string, expires time.Time) string
}

type PathSignatureValidator interface {
	SignatureValid(u *url.URL) bool
}

type PathSignerValidator struct {
	Secret string
	Clock  clock.Clock
}

func (signer *PathSignerValidator) Sign(path string, expires time.Time) string {
	return fmt.Sprintf("%s?md5=%x&expires=%v", path, md5.Sum([]byte(path+signer.Secret)), expires.Unix())
}

func (signer *PathSignerValidator) SignatureValid(u *url.URL) bool {
	expires, e := strconv.ParseInt(u.Query().Get("expires"), 10, 64)
	if e != nil {
		return false
	}
	if signer.Clock.Now().After(time.Unix(expires, 0)) {
		return false
	}

	if u.Query().Get("md5") != fmt.Sprintf("%x", md5.Sum([]byte(u.Path+signer.Secret))) {
		return false
	}
	return true
}
