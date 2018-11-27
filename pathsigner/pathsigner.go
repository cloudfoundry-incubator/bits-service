package pathsigner

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
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
	ActiveKeyID string
}

func Validate(signer *PathSignerValidator) *PathSignerValidator {
	if signer.Secret == "" && len(signer.SigningKeys) == 0 {
		panic(errors.New("must provide either \"Secret\" or \"SigningKeys\" with at least one element"))
	}
	if len(signer.SigningKeys) > 0 && signer.ActiveKeyID == "" {
		panic(errors.New("when providing SigningKeys, you must also provide ActiveKeyID"))
	}
	return signer
}

func (signer *PathSignerValidator) Sign(path string, expires time.Time) string {
	if len(signer.SigningKeys) > 0 {
		return fmt.Sprintf("%s?signature=%x&expires=%v&AccessKeyId=%v", path, signatureWithHMACFor(path, signer.SigningKeys[signer.ActiveKeyID], expires), expires.Unix(), signer.ActiveKeyID)
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

	querySignature, e := hex.DecodeString(u.Query().Get("signature"))
	if e != nil {
		return false
	}

	accessKeyID := u.Query().Get("AccessKeyId")
	if accessKeyID != "" {
		if _, exist := signer.SigningKeys[accessKeyID]; !exist {
			return false
		}
		if subtle.ConstantTimeCompare(querySignature, signatureWithHMACFor(u.Path, signer.SigningKeys[accessKeyID], time.Unix(expires, 0))) == 0 {
			return false
		}
	} else {
		if subtle.ConstantTimeCompare(querySignature, signatureWithHMACFor(u.Path, signer.Secret, time.Unix(expires, 0))) == 0 {
			return false
		}
	}
	return true
}

func signatureWithHMACFor(path string, secret string, expires time.Time) []byte {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(fmt.Sprintf("%v%v %v", expires.Unix(), path, secret)))
	return hash.Sum(nil)
}
