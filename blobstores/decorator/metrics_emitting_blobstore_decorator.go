package decorator

import (
	"io"
	"time"

	"github.com/cloudfoundry-incubator/bits-service"
)

type MetricsEmittingBlobstoreDecorator struct {
	delegate       bitsgo.Blobstore
	metricsService bitsgo.MetricsService
	resourceType   string
}

func ForBlobstoreWithMetricsEmitter(delegate bitsgo.Blobstore, metricsService bitsgo.MetricsService, resourceType string) *MetricsEmittingBlobstoreDecorator {
	return &MetricsEmittingBlobstoreDecorator{delegate, metricsService, resourceType}
}

func (decorator *MetricsEmittingBlobstoreDecorator) Exists(path string) (bool, error) {
	startTime := time.Now()
	exists, e := decorator.delegate.Exists(path)
	decorator.metricsService.SendTimingMetric(decorator.resourceType+"-exists_in_blobstore-time", time.Since(startTime))
	return exists, e
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
	startTime := time.Now()
	e := decorator.delegate.Copy(src, dest)
	decorator.metricsService.SendTimingMetric(decorator.resourceType+"-copy_in_blobstore-time", time.Since(startTime))
	return e
}

func (decorator *MetricsEmittingBlobstoreDecorator) Delete(path string) error {
	startTime := time.Now()
	e := decorator.delegate.Delete(path)
	decorator.metricsService.SendTimingMetric(decorator.resourceType+"-delete_from_blobstore-time", time.Since(startTime))
	return e
}

func (decorator *MetricsEmittingBlobstoreDecorator) DeleteDir(prefix string) error {
	startTime := time.Now()
	e := decorator.delegate.DeleteDir(prefix)
	decorator.metricsService.SendTimingMetric(decorator.resourceType+"-delete_dir_from_blobstore-time", time.Since(startTime))
	return e
}
