package pathsigner

import (
	"crypto/hmac"
	"crypto/sha256"
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
	Secret      string
	Clock       clock.Clock
	SigningKeys map[string]string
}

func (signer *PathSignerValidator) Sign(path string, expires time.Time) string {
	if len(signer.SigningKeys) > 0 {
		var accessKeyID string
		for accessKeyID = range signer.SigningKeys {
			break
		}
		return fmt.Sprintf("%s?signature=%x&expires=%v&AccessKeyId=%v", path, signatureWithHMACFor(path, signer.SigningKeys[accessKeyID], expires), expires.Unix(), accessKeyID)
	}
	return fmt.Sprintf("%s?signature=%x&expires=%v", path, signatureWithHMACFor(path, signer.Secret, expires), expires.Unix())
}

func (signer *PathSignerValidator) SignatureValid(u *url.URL) bool {
	expires, e := strconv.ParseInt(u.Query().Get("expires"), 10, 64)
	if e != nil {
		return false
	}
	if signer.Clock.Now().After(time.Unix(expires, 0)) {
		return false
	}

	accessKeyID := u.Query().Get("AccessKeyId")
	if accessKeyID != "" {
		if _, exist := signer.SigningKeys[accessKeyID]; !exist {
			return false
		}
		if u.Query().Get("signature") != fmt.Sprintf("%x", signatureWithHMACFor(u.Path, signer.SigningKeys[accessKeyID], time.Unix(expires, 0))) {
			return false
		}
	} else {
		if u.Query().Get("signature") != fmt.Sprintf("%x", signatureWithHMACFor(u.Path, signer.Secret, time.Unix(expires, 0))) {
			return false
		}
	}
	return true
}

func signatureWithHMACFor(path string, secret string, expires time.Time) []byte {
	hash := hmac.New(sha256.New, []byte(secret))
	return hash.Sum([]byte(fmt.Sprintf("%v%v %v", expires.Unix(), path, secret)))
}
