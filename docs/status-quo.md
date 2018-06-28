# Cloud Controller Bits Handling - Status Quo

At the moment this document does only cover Cloud Controller API v2.

## Entities

Following are the most important types of entities that are currently being stored in the Cloud Controller's blob store.

### App Bits

Represent parts of an app that has been pushed to CF at some point in the past. It's important to note, that App Bits are not being deleted from the blob store upon `cf delete`. Also, parts that are smaller than a configurable threshold size are not being stored in the blob store.

### Packages

The collection of all current App Bits for a given application.

**Lifecycle**: Deleting an app will delete the blobstore entry.

### Buildpacks

A Cloud Foundry Buildpack that is being used to build a pushed app.

**Lifecycle**: Deleting a buildpack will delete it from the blobstore.

### Buildpacks Cache

Used to store artifacts that are used by a given buildpack (e.g. downloads from the internet like gems a.o.). The cache is specific to a given app.

**Lifecycle**: Blobs for an app will be cleaned when the app is deleted. There is also a bulk delete endpoint.

### Droplets

Droplets are compiled application packages plus additional artifacts injected by the buildpack. A droplet is stored in the blob store after the corresponding app has been staged on a DEA. In order to run the staged app, (another) DEA will retrieve the droplet from the Cloud Controller.

**Lifecycle**: Current and previous versions of a droplet are kept around. See `droplet_uploader.rb`.

## Flows & Request Sequences

This section documents some of the existing flows that involve bit-related HTTP requests to the Cloud Controller. We leave out requests that are not directly related to bits (e.g. requests to create an app route).

### Application-specific

Sequences of bit-related HTTP requests specific to application lifecycle flows.

#### Push Application Without Start

Push an app to CF without staging and starting it (i.e. `cf push --no-start`). There are basically two distinct phases here: first the CLI asks the Cloud Controller to match the app resources, then it uploads all non-matched resources to the Cloud Controller, which in turn will store them in its blob store.

All of the following request are from the CF CLI to the Cloud Controller.

1. `PUT /v2/apps/<app-guid>`

  Creates the app in the Cloud Controller database.

  *Does this already setup the blob store for the app (e.g. create directories/buckets) or does that happen later? Needs more details.*

1. `PUT /v2/resource_match`

  Resource Matching for App Bits

  **Request Body**

  JSON Array in the form of:

  ```
  [
    {"sha1":"0","size":0},
    {"sha1":"ab027c43515d47013ca3b5aacb78552ea801706a","size":2198}, ...
  ]
  ```

  Every entry in the array represents a part of the application (i.e. file).

  **Response Body**

  JSON Array in the form of:

  ```
  [
    {"sha1":"ab027c43515d47013ca3b5aacb78552ea801706a","size":2198},
    ...
  ]
  ```

  Subset of the array in the request body. Each entry represents a part of the application that is already present in the blob store (i.e. matched app parts).

1. `PUT /v2/apps/<app-guid>/bits?async=true`

  Upload non-matched App Bits.

  **Request Body**

  `MULTIPART/FORM-DATA` that represents the contents of all application parts that are not yet present in the blob store (i.e. bits for all non-matched app parts).

  **Request Params**

  `async=true`: Tells the Cloud Controller to asynchronously store the uploaded parts in its blob store and respond right after the multi-part upload finished. This seems to be the default using the CF CLI.

  **Response Body**

  JSON object that represents an asynchronous Job. This job handles all the App Bits that have been uploaded to the Cloud Controller. The most important bit of information in there is the job guid that can be used to poll for the job status in subsequent requests.

  Example:

  ```
  {
    "metadata": {
      "guid": "48033e02-e9d7-427c-86d4-92ae8d226c6f",
      "created_at": "2016-01-26T14:29:30Z",
      "url": "/v2/jobs/48033e02-e9d7-427c-86d4-92ae8d226c6f"
    },
    "entity": {
      "guid": "48033e02-e9d7-427c-86d4-92ae8d226c6f",
      "status": "queued"
    }
  }
  ```

1. `GET /v2/jobs/<job-guid>`

  Ask for job status.

  **Request Body**

  none

  **Response Body**

  JSON object that represents an asynchronous job. Amongst other things this returns the current status of the job (e.g. `finished`, `queued`, ...).

  Example:              

  ```
  {
    "metadata": {
      "guid": "0",
      "created_at": "1970-01-01T00:00:00Z",
      "url": "/v2/jobs/0"
    },
    "entity": {
      "guid": "0",
      "status": "finished"
    }
  }
  ```

  Note that the `url` and `guid` for a finished job are "zero-ed out".

#### Start Application

The following requests are important in terms of bits-handling when a `cf start <app>` is being issued in the CF CLI. These requests are all sent from a DEA to the Cloud Controller. We still have to verify this sequence and more details after looking at the code.

1. `GET /buildpacks/<buildpack-guid>/download` (?)

  Download buildpack bits from Cloud Controller, which in turn will retrieve them from its blob store.

1. `GET /staging/buildpack-cache/<app-guid>/download` (?)

  DEA gets cached buildpack artifacts for the given app.

1. `POST /staging/buildpack-cache/<app-guid>/upload` (?)

  DEA uploads any buildpack artifacts that are not yet stored in the application's buildpack cache.

1. `POST /staging/droplets/<app-guid>/upload`

  DEA uploads a new droplet for the given app to the Cloud Controller. This droplet is the result of the app staging.

1. `GET /staging/droplets/<app-guid>/download`

  DEA downloads the current droplet version for the given app from the Cloud Controller. It uses this to run the app.

#### Push Application With Start

From a bits-handling perspective this is the same as the combination of the request sequences from the previous two sections (`cf push --no-start` plus `cf start <app>`).

#### Delete Application

The following request is sent from the CF CLI to the Cloud Controller.

1. `DELETE /v2/apps/<app-guid>`

  Deletes the app from the Cloud Controller database and removes all droplets from the blob store. *It also looks like the app's buildpack cache is being removed. Need to verify this.*

  **Request Params**

  `async=true`: Remove everything asynchronously. *Is this even be used in the corresponding handler?*

  `recursive=true`: If set to true, the Cloud Controller will remove all service bindings for this app.

  **Request Body**

  todo

  **Response Body**

  todo

#### Copy Application Source

Copies the app bits for a given app to another app inside the blob store. The second app will be restaged after the sources have been copied (see *Start Application*).

1. `POST /v2/apps/<app-guid>/copy_bits`

  Copies app bits from one application to the application identified by the `app-guid` in the route path. This can be initiated through the CF CLI, see `cf copy-source`.

  **Request Body**

  JSON hash that specifies the source app for this copy.

  ```
  {"source_app_guid":"dde976f4-1539-42e0-b559-e8ce06fa5902"}
  ```

  **Request Params**

  `async=true`: The copy happens asynchronously. Cloud Controller provides a job guid for the client to poll on.

  **Response Body**

  JSON object that represents an asynchronous job.

  Example:

  ```
  {
    "metadata": {
      "guid": "4409e8cf-982c-45c3-af91-2b7c19a0a637",
      "created_at": "2016-01-26T17:12:31Z",
      "url": "/v2/jobs/4409e8cf-982c-45c3-af91-2b7c19a0a637"
    },
    "entity": {
      "guid": "4409e8cf-982c-45c3-af91-2b7c19a0a637",
      "status": "queued"
    }
  }
  ```

### Buildpack-specific

Create, update, and delete CF buildpacks. These requests are typically sent from the CF CLI to the Cloud Controller.

#### Create

1. `POST /v2/buildpacks?async=true`

  **Request Body**

  Example:
  ```
  {"name":"myjava","position":21}
  ```

  **Response Body**

  Example:
  ```
  {
    "metadata": {
      "guid": "b76df954-f028-4727-9ba0-5e211af51264",
      "url": "/v2/buildpacks/b76df954-f028-4727-9ba0-5e211af51264",
      "created_at": "2016-01-26T16:45:29Z",
      "updated_at": null
    },
    "entity": {
      "name": "myjava",
      "position": 9,
      "enabled": true,
      "locked": false,
      "filename": null
    }
  }
  ```

1. `PUT /v2/buildpacks/<buildpack-guid>/bits`

  **Request Body**

  `MULTIPART/FORM-DATA` representing the buildpack bits.

  **Response Body**

  ```
  {
    "metadata": {
      "guid": "b76df954-f028-4727-9ba0-5e211af51264",
      "url": "/v2/buildpacks/b76df954-f028-4727-9ba0-5e211af51264",
      "created_at": "2016-01-26T16:45:29Z",
      "updated_at": "2016-01-26T16:45:29Z"
    },
    "entity": {
      "name": "myjava",
      "position": 9,
      "enabled": true,
      "locked": false,
      "filename": "java-buildpack-v3.5.1.zip"
    }
  }
  ```

#### Update

1. `PUT /v2/buildpacks/<app-guid>?async=true`

#### Delete

1. `DELETE /v2/buildpacks/<app-guid>?async=true`

#### Download

See *Start Application*.

### Buildpack Cache

#### Upload

See *Start Application*.

#### Download

See *Start Application*.

#### Bulk Delete

Deletes buildpack caches for all apps. This is an endpoint on the Cloud Controller that can be accessed via `cf curl`.

`DELETE /v2/blobstores/buildpack_cache`

### Droplets

#### Upload

See *Start Application*.

#### Download

See *Start Application*.

In Addition there is a download for droplets in the apps controller.

```
GET /v2/apps/:app-guid/droplet/download
```

#### Delete

See *Start Application*.

### Additional Endpoints

A list of additional endpoints which provide bits handling or are directly related to bits handling but not covered in the flows above.
#### Buildpacks

`GET /internal/buildpacks` provides a list of all buildpacks. DEAs use this endpoint to discover available buildpacks.


## Blob Store Organization

Following holds for the local blobstore. We still have to verify this for S3 a.o.

The blobstore root and directory keys are configurable in the manifest for all  resource types.
* `cc.<resource_type>.fog_connection.local_root`
* `cc.<resource_type>.<resource_type>_directory_key`

Note: it is possible to locate all blobstores in the same directory.

### resources

Root: `cc.droplets.fog_connection.local_root`
Directory Key: `cc.resources.resource_directory_key`
Location: `<root>/<directory_key>/:sha`

### packages

Location: `/:resource_type/:guid`
guid is the guid of the app (v2)

### buildpacks

Location: `/:resourcetype/:guid:sha`
guid is the guid of the buildpack

### droplets

Location: `/:resource_type/:guid/:sha`
V2: guid is the guid of the app
V3: guid is the guid of the droplet

### droplets - buildpackcache (part of droplets)

Location: `/:resource_type/buildpack_cache/:guid/:filename`
filename appears to be related to the stack

## Notes

* There is an open issue on CC for orphan cleanup in the blobstore
** [100GB of orphan packages & droplets in blobstore](https://github.com/cloudfoundry/cloud_controller_ng/issues/440)
** [PR for packages](https://github.com/cloudfoundry/cloud_controller_ng/pull/461)
