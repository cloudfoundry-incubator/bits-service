# Performance Comparison

## Speed

The performance test was done in an AWS environment with an S3 blobstore backend. The bits-service was deployed as one VM with the Ruby implementation in it, and another VM with the Go implementation in it.

### GET requests

GET requests are about **3 orders of magnitude faster** in this Go implementation than in the [original Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service).

#### 100 requests with concurrency 10:

![](images/GET-request-speed-comparison-100-10.png)

#### 10 requests with concurrency 1:

![](images/GET-request-speed-comparison-10-1.png)

#### 100 requests with concurrency 100:

![](images/GET-request-speed-comparison-100-100.png)

### PUT requests

PUT requests are about **2 orders of magnitude faster** in this Go implementation than in the [original Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service) when doing 100 requests with concurrency 10:

![](images/PUT-request-speed-comparison.png)

## Memory Consumption

### Ruby Implementation

The [Ruby implementation](https://github.com/cloudfoundry-incubator/bits-service) uses more or less by default 1GB of memory.

![](images/bits-service-ruby-mem-consumption.png)

### Go

TBD

