package decorator

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/bits-service"
)

type MetricsEmittingBlobstoreDecorator struct {
	delegate       Blobstore
	metricsService bitsgo.MetricsService
	resourceType   string
}

func ForBlobstoreWithMetricsEmitter(delegate Blobstore, metricsService bitsgo.MetricsService, resourceType string) *MetricsEmittingBlobstoreDecorator {
	return &MetricsEmittingBlobstoreDecorator{delegate, metricsService, resourceType}
}

func (decorator *MetricsEmittingBlobstoreDecorator) Exists(path string) (bool, error) {
	return decorator.delegate.Exists(path)
}

func (decorator *MetricsEmittingBlobstoreDecorator) HeadOrRedirectAsGet(path string) (redirectLocation string, err error) {
	return decorator.delegate.HeadOrRedirectAsGet(path)
}

func (decorator *MetricsEmittingBlobstoreDecorator) Get(path string) (body io.ReadCloser, err error) {
	return decorator.delegate.Get(path)
}

func (decorator *MetricsEmittingBlobstoreDecorator) GetOrRedirect(path string) (body io.ReadCloser, redirectLocation string, err error) {
	return decorator.delegate.GetOrRedirect(path)
}

func (decorator *MetricsEmittingBlobstoreDecorator) Put(path string, src io.ReadSeeker) error {
	startTime := time.Now()
	e := decorator.delegate.Put(path, src)
	decorator.metricsService.SendTimingMetric(decorator.resourceType+"-cp_to_blobstore-time", time.Since(startTime))
	return e
}

func (decorator *MetricsEmittingBlobstoreDecorator) Copy(src, dest string) error {
	return decorator.delegate.Copy(src, dest)
}

func (decorator *MetricsEmittingBlobstoreDecorator) Delete(path string) error {
	return decorator.delegate.Delete(path)
}

func (decorator *MetricsEmittingBlobstoreDecorator) DeleteDir(prefix string) error {
	return decorator.delegate.DeleteDir(prefix)
}
