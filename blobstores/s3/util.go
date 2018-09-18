package s3

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"go.uber.org/zap"
)

var loglevelTypes = map[string]aws.LogLevelType{
	"LogDebug":                   aws.LogDebug,
	"LogDebugWithSigning":        aws.LogDebugWithSigning,
	"LogDebugWithHTTPBody":       aws.LogDebugWithHTTPBody,
	"LogDebugWithRequestRetries": aws.LogDebugWithRequestRetries,
	"LogDebugWithRequestErrors":  aws.LogDebugWithRequestErrors,
}

func newS3Client(region string, useIAMProfile bool, accessKeyID string, secretAccessKey string, host string, logger *zap.SugaredLogger, loglevelString string) *s3.S3 {
	c := &aws.Config{
		Region:   aws.String(region),
		Endpoint: aws.String(host),
	}
	if !useIAMProfile {
		c.Credentials = credentials.NewStaticCredentials(accessKeyID, secretAccessKey, "")
	}
	if loglevelString != "" {
		c.Logger = aws.LoggerFunc(func(args ...interface{}) { logger.Debug(args...) })

		if loglevel, exist := loglevelTypes[loglevelString]; exist {
			c.LogLevel = aws.LogLevel(loglevel)
			logger.Infow("Enabled S3 debug log", "log-level", loglevelString)
		} else {
			c.LogLevel = aws.LogLevel(aws.LogDebug)
			logger.Errorw("Invalid S3 debug loglevel. Using default S3 log-level", "log-level", loglevelString, "default-log-level", "LogDebug")
		}
	}
	s, e := session.NewSession(c)
	if awsErr, isAwsErr := e.(awserr.Error); isAwsErr {
		if awsErr.Code() == "NoCredentialProviders" && useIAMProfile {
			logger.Fatalw("Blobstore is configured to use EC2 instance roles (use-iam-profiles), but no EC2 instance role could be found. "+
				"Please make sure that an EC2 instance role is attached to the EC2 instance this service is running on.",
				"use-iam-profiles", useIAMProfile,
				"access-key-id", accessKeyID,
				"secret-access-key-is-set", secretAccessKey != "",
				"region", region,
				"host", host)
		}
	}
	if e != nil {
		logger.Fatalw("Error while trying to create AWS client", "error", e)
	}
	return s3.New(s)
}

func isS3NotFoundError(e error) bool {
	if ae, isAwsErr := e.(awserr.Error); isAwsErr {
		if ae.Code() == "NoSuchKey" || ae.Code() == "NotFound" {
			return true
		}
	}
	return false
}

func isS3NoSuchBucketError(e error) bool {
	if ae, isAwsErr := e.(awserr.Error); isAwsErr {
		if ae.Code() == "NoSuchBucket" {
			return true
		}
	}
	return false
}
