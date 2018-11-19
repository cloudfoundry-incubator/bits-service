package signer

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
)

type V2Signer struct {
	Request     *http.Request
	ExpireTime  time.Duration
	Credentials *credentials.Credentials
	Debug       aws.LogLevelType
	Logger      aws.Logger
	Bucket      string
}

func (v2 V2Signer) Sign(req *request.Request) {
	if req.Config.Credentials == credentials.AnonymousCredentials {
		return
	}
	v2.Request = req.HTTPRequest
	v2.ExpireTime = req.ExpireTime

	req.Error = v2.sign()
}

func (v2 *V2Signer) sign() error {
	credValue, err := v2.Credentials.Get()
	if err != nil {
		return err
	}

	expireTime := time.Now().Add(v2.ExpireTime)

	var keys []string
	for k := range v2.Request.Header {
		keys = append(keys, k)
	}
	sort.StringSlice(keys).Sort()

	var sarray []string
	for _, k := range keys {
		if strings.HasPrefix(strings.ToLower(k), "x-amz-") && strings.ToLower(k) != "x-amz-date" {
			sarray = append(sarray, strings.ToLower(k)+":"+strings.Join(v2.Request.Header[k], ","))
		}
	}
	canonicalizedAmzHeaders := ""
	if len(sarray) > 0 {
		canonicalizedAmzHeaders = strings.Join(sarray, "\n") + "\n"
	}

	stringToSign := ""
	if v2.ExpireTime > 0 {
		stringToSign = strings.Join([]string{
			v2.Request.Method,
			v2.Request.Header.Get("Content-MD5"),
			v2.Request.Header.Get("Content-Type"),
			fmt.Sprintf("%v", expireTime.Unix()),
			canonicalizedAmzHeaders +
				"/" + v2.Bucket + strings.Replace(v2.Request.URL.EscapedPath(), "%2F", "/", -1),
		}, "\n")
	} else {
		stringToSign = strings.Join([]string{
			v2.Request.Method,
			v2.Request.Header.Get("Content-MD5"),
			v2.Request.Header.Get("Content-Type"),
			expireTime.UTC().Format(http.TimeFormat),
			canonicalizedAmzHeaders +
				"/" + v2.Bucket + strings.Replace(v2.Request.URL.EscapedPath(), "%2F", "/", -1),
		}, "\n")
	}

	hash := hmac.New(sha1.New, []byte(credValue.SecretAccessKey))
	hash.Write([]byte(stringToSign))
	signature := string(base64.StdEncoding.EncodeToString(hash.Sum(nil)))

	if v2.ExpireTime > 0 {
		params := v2.Request.URL.Query()
		params["Expires"] = []string{fmt.Sprintf("%v", expireTime.Unix())}
		params["AWSAccessKeyId"] = []string{credValue.AccessKeyID}
		params["Signature"] = []string{signature}
		v2.Request.URL.RawQuery = url.Values(params).Encode()
	} else {
		v2.Request.Header.Set("Authorization", "AWS "+credValue.AccessKeyID+":"+signature)
		v2.Request.Header.Set("Date", expireTime.UTC().Format(http.TimeFormat))
	}

	if v2.Debug.Matches(aws.LogDebugWithSigning) {
		v2.Logger.Log(fmt.Sprintf(logSignInfoMsg, stringToSign, v2.Request.URL.String()))
	}
	return nil
}

const logSignInfoMsg = `DEBUG: Request Signature:
---[ STRING TO SIGN ]--------------------------------
%s
---[ SIGNED URL ]------------------------------------
%s`
