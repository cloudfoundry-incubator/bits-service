# Performance Comparison

## Speed

The performance test was done in an AWS environment with an S3 blobstore backend.

### GET requests

GET requests are about **3 orders of magnitude faster** in this Go implementation than in the [original Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service).

#### 100 requests with concurrency 10:

![](GET-request-speed-comparison-100-10.png)

#### 10 requests with concurrency 1:

![](GET-request-speed-comparison-10-1.png)

#### 100 requests with concurrency 100:

![](GET-request-speed-comparison-100-100.png)

### PUT requests

PUT requests are about **2 orders of magnitude faster** in this Go implementation than in the [original Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service) when doing 100 requests with concurrency 10:

![](PUT-request-speed-comparison.png)

## Memory Consumption

### Ruby Implementation

The [Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service) uses more or less by default 1GB of memory.

![](bits-service-ruby-mem-consumption.png)

### Go

TBD

