#!/bin/bash -xe

BLOBSTORE_TYPE="${1:?Missing parameter indicating blobstore type
USAGE: run-contract-integ-tests.sh <azure|S3|GCP|openstack|alibaba>}"

ginkgo -r --focus=$BLOBSTORE_TYPE -skip='SLOW TESTS'
