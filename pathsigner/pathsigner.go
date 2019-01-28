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
	"strings"
	"time"

	"github.com/benbjohnson/clock"
)

type PathSigner interface {
	Sign(method string, path string, expires time.Time) string
}

type PathSignatureValidator interface {
	SignatureValid(method string, u *url.URL) bool
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

func (signer *PathSignerValidator) Sign(method string, path string, expires time.Time) string {
	method = strings.ToUpper(method)
	if len(signer.SigningKeys) > 0 {
		return fmt.Sprintf("%s?signature=%x&expires=%v&AccessKeyId=%v", path, signatureWithHMACFor(method, path, signer.SigningKeys[signer.ActiveKeyID], expires), expires.Unix(), signer.ActiveKeyID)
	}
	return fmt.Sprintf("%s?signature=%x&expires=%v", path, signatureWithHMACFor(method, path, signer.Secret, expires), expires.Unix())
}

func (signer *PathSignerValidator) SignatureValid(method string, u *url.URL) bool {
	method = strings.ToUpper(method)
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
		if subtle.ConstantTimeCompare(querySignature, signatureWithHMACFor(method, u.Path, signer.SigningKeys[accessKeyID], time.Unix(expires, 0))) == 0 {
			return false
		}
	} else {
		if subtle.ConstantTimeCompare(querySignature, signatureWithHMACFor(method, u.Path, signer.Secret, time.Unix(expires, 0))) == 0 {
			return false
		}
	}
	return true
}

func signatureWithHMACFor(method string, path string, secret string, expires time.Time) []byte {
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(fmt.Sprintf("%v %v %v %v", method, path, secret, expires.Unix())))
	return hash.Sum(nil)
}
