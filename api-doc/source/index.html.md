---
title: Bits-Service API Reference

language_tabs:
  - curl

search: true
---

# Introduction

The bits-service is an extraction from existing functionality of the [cloud controller](https://github.com/cloudfoundry/cloud_controller_ng). It encapsulates all "bits operations" into its own, separately scalable service. All bits operations comprise buildpacks, droplets, app_stashes, packages and the buildpack_cache.

### Supported Backends
Bits currently supports [WebDAV](https://en.wikipedia.org/wiki/WebDAV) and the following [Fog](http://fog.io/) connectors:

* AWS S3
* Azure
* Google
* Local (NFS)
* Openstack

# Packages

A package are the files that make up an application from the developer's point of view (source code).

## Uploading a Package

> Example request:

```shell
curl -X PUT 'https://internal.example.com/packages/c33e184b-e698-4290-952e-4047601e4627' \
  -F package=@package-file
```

> Example response:

```shell
HTTP/1.1 201 Created
```

### HTTP Request
`PUT /packages/:guid`

where `:guid` is the package's GUID.

### Request Body
`package: <formfile>`

If the body is not a file upload, but contains a `:source_guid`, its value is treated as `:guid` and an attempt is made to copy the package from the one identified by the value of `:source_guid`.

### Access
Internal endpoint only

## Downloading a Package

> Example request:

```shell
curl -X GET 'https://internal.example.com/packages/c33e184b-e698-4290-952e-4047601e4627'
```

> Example response when backend is S3:

```shell
HTTP/1.1 302 Found
```

> Example response when backend is local:

```shell
HTTP/1.1 200 OK

<file contents>
```

### HTTP Request
`GET /packages/:guid`

where `:guid` is the package's GUID.

### Access
Internal endpoint only

## Deleting a Package

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/packages/c33e184b-e698-4290-952e-4047601e4627'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /packages/:guid`

where `:guid` is the package's GUID.

### Access
Internal endpoint only

# Droplets

A droplet is the result of staging an application package. It contains the bits produced by the buildpack, typically application code and dependencies.

## Uploading a Droplet

> Example request:

```shell
curl -X PUT 'https://internal.example.com/droplets/c33e184b-e698-4290-952e-4047601e4627/b1d2a97c5033319632e65beba49dd92da18c1d20' \
  -F droplet=@droplet-file
```

> Example response:

```shell
HTTP/1.1 201 Created
```

### HTTP Request
`PUT /droplets/:guid/:checksum`

where `:guid` is the droplet's GUID and `:checksum` is its checksum.

### Request Body
`droplet: <formfile>`

If the body is not a file upload, but contains a `:source_guid`, its value is treated as `:guid` and an attempt is made to copy the droplet from the one identified by the value of `:source_guid`.

### Access
Internal endpoint only

## Uploading a Droplet with Digest in Header

> Example request:

```shell
curl --request PUT --header 'Digest: sha256=abcdefg' --data-binary @droplet-file 'https://example.com/signed/droplets/4facf67a-2880-4367-928e-b4c88f63bcda?md5=tDTwS8DEdA0T-b0RRx_TIw&expires=1510839810'
```

> Example response:

```shell
HTTP/1.1 201 Created
```

### HTTP Request
`PUT /droplets/:guid`

where `:guid` is the droplet's GUID.

### Request Headers

`Digest: sha256=abcdefg`

The format of the `Digest` header's value is `<Algorithm>=<Value>`. Currently only `sha256` is supported.

### Request Body

`content-of-droplet-file`

The body will always be treated as `application/octet-stream`.

### Access

This endpoint is public and can only be used with a signed URL.

## Downloading a Droplet

> Example request:

```shell
curl -X GET 'https://internal.example.com/droplets/c33e184b-e698-4290-952e-4047601e4627/b1d2a97c5033319632e65beba49dd92da18c1d20'
```

> Example response when backend is S3:

```shell
HTTP/1.1 302 Found
```

> Example response when backend is local:

```shell
HTTP/1.1 200 OK

<file contents>
```

### HTTP Request
`GET /droplets/:guid/:checksum`

where `:guid` is the droplet's GUID and `:checksum` is its checksum.

### Access
Internal endpoint only

## Deleting a Droplet

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/droplets/c33e184b-e698-4290-952e-4047601e4627/b1d2a97c5033319632e65beba49dd92da18c1d20'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /droplets/:guid/:checksum`

where `:guid` is the droplet's GUID and `:checksum` is its checksum.

### Access
Internal endpoint only

# Buildpacks

A buildpack provides the components necessary to run an application, e.g. the compiler or interpreter for the source code of an app, and often times also an application framework.

## Uploading a Buildpack

> Example request:

```shell
curl -X PUT 'https://internal.example.com/buildpacks/c33e184b-e698-4290-952e-4047601e4627' \
  -F buildpack=@buildpack-file
```

> Example response:

```shell
HTTP/1.1 201 Created
```

### HTTP Request
`PUT /buildpacks/:guid`

where `:guid` is the buildpack's GUID.

### Request Body
`buildpack: <formfile>`

### Access
Internal endpoint only

## Downloading a Buildpack

> Example request:

```shell
curl -X GET 'https://internal.example.com/buildpacks/c33e184b-e698-4290-952e-4047601e4627'
```

> Example response when backend is S3:

```shell
HTTP/1.1 302 Found
```

> Example response when backend is local:

```shell
HTTP/1.1 200 OK

<file contents>
```

### HTTP Request
`GET /buildpacks/:guid`

where `:guid` is the buildpack's GUID.

### Access
Internal endpoint only

## Deleting a Buildpack

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/buildpacks/c33e184b-e698-4290-952e-4047601e4627'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /buildpacks/:guid`

where `:guid` is the buildpack's GUID.

### Access
Internal endpoint only

# Buildpack Cache Entries

A buildpack may choose to cache certain dependencies of an app (e.g. Node modules or Ruby gems). These will be stored as buildpack cache entries.

## Uploading a Buildpack Cache Entry

> Example request:

```shell
curl -X PUT 'https://internal.example.com/buildpack_cache/entries/83d28f59-d3f7-4d00-9a10-459a69649a87/cflinux' \
  -F buildpack_cache=@buildpack-cache
```

> Example response:

```shell
HTTP/1.1 201 Created
```

### HTTP Request
`PUT /buildpack_cache/entries/:guid/:stack_name`

where `:guid` is the GUID of the app this buildpack cache is maintained for. `:stack_name` is the name of the stack the app is running under, e.g. `cflinux`.

### Request Body
`buildpack_cache: <formfile>`

### Access
Internal endpoint only

## Downloading a Buildpack Cache Entry

> Example request:

```shell
curl -X GET 'https://internal.example.com/buildpack_cache/entries/83d28f59-d3f7-4d00-9a10-459a69649a87/cflinux'
```

> Example response when backend is S3:

```shell
HTTP/1.1 302 Found
```

> Example response when backend is local:

```shell
HTTP/1.1 200 OK

<file contents>
```

### HTTP Request
`GET /buildpack_cache/entries/:guid/:stack_name`

where `:guid` is the GUID of the app this buildpack cache is maintained for. `:stack_name` is the name of the stack the app is running under, e.g. `cflinux`.

### Access
Internal endpoint only

## Deleting a Buildpack Cache Entry

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/buildpack_cache/entries/83d28f59-d3f7-4d00-9a10-459a69649a87/cflinux'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /buildpack_cache/entries/:guid/:stack_name`

where `:guid` is the GUID of the app this buildpack cache is maintained for. `:stack_name` is the name of the stack the app is running under, e.g. `cflinux`.

### Access
Internal endpoint only

## Deleting all Buildpack Cache Entries for an app

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/buildpack_cache/entries/83d28f59-d3f7-4d00-9a10-459a69649a87'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /buildpack_cache/entries/:guid`

where `:guid` is the GUID of the app this buildpack cache is maintained for.

### Access
Internal endpoint only

## Deleting all Buildpack Cache Entries

> Example request:

```shell
curl -X DELETE 'https://internal.example.com/buildpack_cache/entries'
```

> Example response:

```shell
HTTP/1.1 204 No Content
```

### HTTP Request
`DELETE /buildpack_cache/entries`

### Access
Internal endpoint only

# App Stash

App Stash optimizes the repeated app push, so that unchanged files need not to be uploaded more than once. It acts like a cache to which files can be uploaded and later referred to in order to bundle those files into a package.

## Matching Entries

> Example request:

```shell
curl -X POST 'https://internal.example.com/app_stash/matches' \
    -d '[{
          "sha1": "8b381f8864b572841a26266791c64ae97738a659",
          "size": 534567
        },
        {
          "sha1": "594eb15515c89bbfb0874aa4fd4128bee0a1d0b5",
          "size": 9874
        }]'
```

> Given that the following files are present in app stash:

```shell
Size | Filename / Checksum
-----|-----------------------------------------
9874 | 594eb15515c89bbfb0874aa4fd4128bee0a1d0b5
6787 | 987348957349857349349haf6876786ehg909034
1029 | abddd9587agbacfbab98d9890908a8979bbb7898
```

> Example response:

```shell
HTTP/1.1 200 OK

[{
  "sha1": "594eb15515c89bbfb0874aa4fd4128bee0a1d0b5",
  "size": 9874
}]
```

This endpoint matches a list of file entries with entries already in the blobstore app stash and returns the ones that are already there.

### HTTP Request
`POST /app_stash/matches`

### Body Parameters
JSON array with elements as in `{"sha1": "<sha1-checksum>", "size": <file-size>}`.

### Access
Internal endpoint only

## Uploading Entries

> Example request:

```shell
curl -X POST 'https://internal.example.com/app_stash/entries' \
  -F application=@entries
```

> Example response:

```shell
HTTP/1.1 201 Created

[{
  "sha1": "8b381f8864b572841a26266791c64ae97738a659",
  "fn":   "script.rb",
  "mode": "0644"
}]
```

This endpoint takes a zip file and stores its uncompressed files in the app stash.

### HTTP Request
`POST /app_stash/entries`

### Body Parameters
`application: <formfile>`

### Access
Internal endpoint only

## Bundling Entries

> Example request:

```shell
curl -X POST 'https://internal.example.com/app_stash/bundles' \
     -d '[{
           "sha1": "8b381f8864b572841a26266791c64ae97738a659",
           "fn":   "script.rb",
           "mode": "755"
         },
         {
           "sha1": "594eb15515c89bbfb0874aa4fd4128bee0a1d0b5",
           "fn":   "lib/backend.rb",
           "mode": "644"
         }]'
```

> Example response:

```shell
HTTP/1.1 200 OK

<zip file with requested entries>
```

This endpoint creates and returns a zip file by bundling file entries from the app stash. The entries are defined in the body parameters.

### HTTP Request
`POST /app_stash/bundles`

### Body Parameters
JSON array with elements as in `{"sha1": "<sha1-checksum>", "fn": "<filename>", "mode": "<filemode>"}`.

### Access
Internal endpoint only

# Signed URLs

In order to prevent leakage of resources, all external access to the Bits-Service must be done using signed URLs. Signing usually requires username and password.

## Signing a URL

> Example request:

```shell
curl 'https://username:password@internal.example.com/sign/packages/bdf47b84-1349-4abd-9561-5004858dfa05?verb=put'
```

> Example response:

```shell
HTTP/1.1 200 OK

https://bits-service.example.com/signed/packages/test-package?md5=yBh47LwYRQ4d8SG6mNsL4w&expires=1497357804
```

### HTTP Request
`GET /sign/:path`

where `:path` is the URL path of the signed entity.

### Query Parameters
Parameter | Default | Description
--------- | ------- | -----------
`verb`    | `GET`   | Defines the verb that can be used in association with the signed URL. Either `GET` or `PUT`.

### Access
Internal endpoint only

> Example signed upload:

```shell
curl -X PUT 'https://internal.example.com/signed/packages/bdf47b84-1349-4abd-9561-5004858dfa05?md5=YDjcMjytsnVEzoSxqpiC4A&expires=1492594615' \
  -F package=@package-file
```

<aside class="notice">
Signing URL does not imply that the resource exists.
</aside>

# Metrics

The bits-service emits the following metrics:

## Response times

### Status code agnostic

These are of the form `bits.<request-method>-<resource-type>-time`, e.g.:

* `bits.PUT-packages-time`
* `bits.DELETE-droplets-time`
* `bits.POST-app_stash-time`

### By status code

These are of the form `bits.<request-method>-<resource-type>-<status-code>-time`, e.g.:

* `bits.PUT-packages-403-time`
* `bits.DELETE-droplets-200-time`
* `bits.POST-app_stash-500-time`

## Response sizes

Similar to response times, these are of the form `bits.<request-method>-<resource-type>-size`, e.g.:

* `bits.GET-buildpacks-size`
* `bits.DELETE-droplets-size`
* `bits.POST-app_stash-size`

While they are available for all requests, these are most interesting for `GET` requests.
## Request sizes

Similar to response times, these are of the form `bits.<request-method>-<resource-type>-request-size`, e.g.:

* `bits.GET-buildpacks-request-size`
* `bits.DELETE-droplets-request-size`
* `bits.POST-app_stash-request-size`

While they are available for all requests, these are most interesting for `PUT`/`POST` requests.

## Request status codes

These are of the form `bits.status-<status-code>`, e.g.:

* `bits.status-404`
* `bits.status-200`

## Copying bits to the blobstore

* `bits.app_stash-cp_r_to_blobstore-time`
* `bits.packages-cp_to_blobstore-time`
* `bits.buildpack-cp_to_blobstore-time`
* `bits.droplet-cp_to_blobstore-time`
* `bits.buildpack_cache-cp_to_blobstore-time`

## Updating the Cloud Controller

* `bits.packages-cc_updater_processing_upload-time`
* `bits.packages-cc_updater_ready-time`
* `bits.packages-cc_updater_failed-time`
