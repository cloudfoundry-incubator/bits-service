package s3

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cloudfoundry-incubator/bits-service/blobstores/s3/signer"
	"go.uber.org/zap"
)

var loglevelTypes = map[string]aws.LogLevelType{
	"LogDebug":                    aws.LogDebug,
	"LogDebugWithSigning":         aws.LogDebugWithSigning,
	"LogDebugWithHTTPBody":        aws.LogDebugWithHTTPBody,
	"LogDebugWithRequestRetries":  aws.LogDebugWithRequestRetries,
	"LogDebugWithRequestErrors":   aws.LogDebugWithRequestErrors,
	"LogDebugWithEventStreamBody": aws.LogDebugWithEventStreamBody,
}

func newS3Client(region string,
	useIAMProfile bool,
	accessKeyID string,
	secretAccessKey string,
	host string,
	logger *zap.SugaredLogger,
	loglevelString string,
	bucket string,
	signatureVersion int,
) *s3.S3 {
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
	ses := session.Must(session.NewSession(c))
	s3Client := s3.New(ses)

	if signatureVersion == 2 {
		s3Client.Handlers.Sign.Swap(v4.SignRequestHandler.Name, request.NamedHandler{
			Name: "v2.SignHandler",
			Fn: (&signer.V2Signer{
				Credentials: c.Credentials,
				Debug:       *ses.Config.LogLevel,
				Logger:      ses.Config.Logger,
				Bucket:      bucket,
			}).Sign,
		})
		logger.Infow("Using signature version 2 signing.")
	}

	// This priming is only done to make the service fail fast in case it was misconfigured instead of making it fail on the first request served.
	_, e := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String("dummy"),
		Key:    aws.String("dummy"),
	})
	if awsErr, isAwsErr := e.(awserr.Error); isAwsErr && awsErr.Code() == "NoCredentialProviders" && useIAMProfile {
		logger.Fatalw("Blobstore is configured to use EC2 instance roles (use-iam-profiles), but no EC2 instance role could be found. "+
			"If you want to use EC2 instance roles, please make sure that an EC2 instance role is attached to the EC2 instance this service is running on. "+
			"No access-key-id and no secret-access-key is needed in that case. See also: https://docs.cloudfoundry.org/deploying/common/cc-blobstore-config.html#fog-aws-iam.",
			"use-iam-profiles", useIAMProfile,
			"access-key-id", accessKeyID,
			"secret-access-key-is-set", secretAccessKey != "",
			"region", region,
			"host", host)
	}
	return s3Client
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
