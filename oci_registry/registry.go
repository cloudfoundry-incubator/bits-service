package oci_registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	bitsgo "github.com/cloudfoundry-incubator/bits-service"

	"github.com/cloudfoundry-incubator/bits-service/oci_registry/models/docker"
	"github.com/cloudfoundry-incubator/bits-service/oci_registry/models/docker/mediatype"
	"github.com/cloudfoundry-incubator/bits-service/util"

	"github.com/gorilla/mux"
)

type ImageHandler struct {
	ImageManager *BitsImageManager
}

func (m *ImageHandler) ServeAPIVersion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Pong"))
}

func (m *ImageHandler) ServeManifest(w http.ResponseWriter, r *http.Request) {
	// TODO (pego): this is a hack to address to quickly find out if this should serve a manifest or manifest list. Should be improved.
	if m.ImageManager.GetBlob("not used", mux.Vars(r)["tag"]) != nil {
		mux.Vars(r)["digest"] = mux.Vars(r)["tag"]
		m.ServeBlob(w, r)
		return
	}

	manifestList := m.ImageManager.GetManifestList(strings.TrimPrefix(mux.Vars(r)["name"], "cloudfoundry/"), mux.Vars(r)["tag"])

	if manifestList == nil {
		http.NotFound(w, r)
		return
	}

	manifestListJson, e := json.Marshal(manifestList)
	util.PanicOnError(errors.WithStack(e))

	manifestListDigest, manifestListSize := shaAndSize(bytes.NewReader(manifestListJson))
	e = m.ImageManager.digestLookupStore.Put(manifestListDigest, bytes.NewReader(manifestListJson))
	util.PanicOnError(errors.WithStack(e))

	w.Header().Add("Content-Type", mediatype.DistributionManifestListV2Json)
	w.Header().Add("Docker-Content-Digest", manifestListDigest)
	w.Header().Add("Content-Length", fmt.Sprintf("%d", manifestListSize))

	w.Write(manifestListJson)
}

func (m *ImageHandler) ServeBlob(w http.ResponseWriter, r *http.Request) {
	digest := mux.Vars(r)["digest"]
	layer := m.ImageManager.GetBlob(mux.Vars(r)["name"], digest)

	if layer == nil {
		http.NotFound(w, r)
		return
	}

	// TODO (pego): this is a hack to find out if we should serve a layer or a manifest blob. Should be improved.
	if mux.Vars(r)["digest"] == mux.Vars(r)["tag"] {
		w.Header().Add("Content-Type", mediatype.DistributionManifestV2Json)
	} else {
		w.Header().Add("Content-Type", mediatype.ImageRootfsTarGzip)
	}

	w.Header().Add("Docker-Content-Digest", digest)
	_, e := io.Copy(w, layer)

	util.PanicOnError(errors.WithStack(e))
}

type BitsImageManager struct {
	rootFSBlobstore   bitsgo.Blobstore
	dropletBlobstore  bitsgo.Blobstore
	digestLookupStore bitsgo.Blobstore
	rootfsSize        int64
	rootfsDigest      string
}

func NewBitsImageManager(
	rootFSBlobstore bitsgo.Blobstore,
	dropletBlobstore bitsgo.Blobstore,
	digestLookupStore bitsgo.Blobstore) *BitsImageManager {

	rootfsReader, e := rootFSBlobstore.Get("assets/eirinifs.tar")
	if bitsgo.IsNotFoundError(e) {
		panic(errors.New("Could not find assets/eirinifs.tar in root FS blobstore. " +
			"Please make sure that copy it to the root FS blobstore as part of your deployment."))
	}
	util.PanicOnError(errors.WithStack(e))
	rootfsDigest, rootfsSize := shaAndSize(rootfsReader)

	return &BitsImageManager{
		rootFSBlobstore:   rootFSBlobstore,
		dropletBlobstore:  dropletBlobstore,
		digestLookupStore: digestLookupStore,
		rootfsSize:        rootfsSize,
		rootfsDigest:      rootfsDigest,
	}
}

func (b *BitsImageManager) GetManifestList(dropletGUID string, dropletHash string) *docker.ManifestList {
	manifest := b.GetManifest(dropletGUID, dropletHash)
	if manifest == nil {
		return nil
	}
	manifestJson, e := json.Marshal(manifest)
	util.PanicOnError(errors.WithStack(e))

	manifestDigest, manifestSize := shaAndSize(bytes.NewReader(manifestJson))

	e = b.digestLookupStore.Put(manifestDigest, bytes.NewReader(manifestJson))
	util.PanicOnError(errors.WithStack(e))

	return &docker.ManifestList{
		Versioned: docker.Versioned{
			MediaType:     mediatype.DistributionManifestListV2Json,
			SchemaVersion: 2,
		},
		Manifests: []docker.ManifestDescriptor{
			docker.ManifestDescriptor{
				Content: docker.Content{
					MediaType: mediatype.DistributionManifestV2Json,
					Size:      manifestSize,
					Digest:    manifestDigest,
				},
				Platform: docker.PlatformSpec{
					Architecture: "amd64",
					OS:           "linux",
				},
			},
		},
	}
}

func (b *BitsImageManager) GetManifest(dropletGUID string, dropletHash string) *docker.Manifest {
	dropletReader, e := b.dropletBlobstore.Get(dropletGUID + "/" + dropletHash)

	if bitsgo.IsNotFoundError(e) {
		return nil
	}
	util.PanicOnError(errors.WithStack(e))
	defer dropletReader.Close()

	ociDropletFile, e := ioutil.TempFile("", "oci-droplet")
	util.PanicOnError(errors.WithStack(e))

	defer os.Remove(ociDropletFile.Name())
	defer ociDropletFile.Close()

	preFixDroplet(dropletReader, ociDropletFile)

	_, e = ociDropletFile.Seek(0, 0)
	util.PanicOnError(errors.WithStack(e))

	dropletDigest, dropletSize := shaAndSize(ociDropletFile)

	_, e = ociDropletFile.Seek(0, 0)
	util.PanicOnError(errors.WithStack(e))

	e = b.digestLookupStore.Put(dropletDigest, ociDropletFile)
	util.PanicOnError(errors.WithStack(e))

	configJSON := b.configMetadata(b.rootfsDigest, dropletDigest)
	configDigest, configSize := shaAndSize(bytes.NewReader(configJSON))

	e = b.digestLookupStore.Put(configDigest, bytes.NewReader(configJSON))
	util.PanicOnError(errors.WithStack(e))

	return &docker.Manifest{
		Versioned: docker.Versioned{
			MediaType:     mediatype.DistributionManifestV2Json,
			SchemaVersion: 2,
		},
		Config: docker.Content{
			MediaType: mediatype.ContainerImageV1Json,
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: []docker.Content{
			docker.Content{
				MediaType: mediatype.ImageRootfsTarGzip,
				Digest:    b.rootfsDigest,
				Size:      b.rootfsSize,
			},
			docker.Content{
				MediaType: mediatype.ImageRootfsTar,
				Digest:    dropletDigest,
				Size:      dropletSize,
			},
		},
	}
}

func preFixDroplet(cfDroplet io.Reader, ociDroplet io.Writer) {
	layer := tar.NewWriter(ociDroplet)

	gz, e := gzip.NewReader(cfDroplet)
	util.PanicOnError(errors.WithStack(e))

	t := tar.NewReader(gz)
	for {
		hdr, e := t.Next()
		if e == io.EOF {
			break
		}
		util.PanicOnError(errors.WithStack(e))

		hdr.Name = filepath.Join("/home/vcap", hdr.Name)
		hdr.Mode = 0777
		hdr.Uname = "vcap"
		hdr.Gname = "vcap"
		e = layer.WriteHeader(hdr)
		util.PanicOnError(errors.WithStack(e))
		_, e = io.Copy(layer, t)
		util.PanicOnError(errors.WithStack(e))
	}
}

func shaAndSize(reader io.Reader) (sha string, size int64) {
	sha256Hash := sha256.New()
	configSize, e := io.Copy(sha256Hash, reader)
	util.PanicOnError(errors.WithStack(e))
	return "sha256:" + hex.EncodeToString(sha256Hash.Sum([]byte{})), configSize
}

// NOTE: name is currently not used.
func (b *BitsImageManager) GetBlob(name string, digest string) io.ReadCloser {
	if digest == b.rootfsDigest {
		r, e := b.rootFSBlobstore.Get("assets/eirinifs.tar")
		util.PanicOnError(errors.WithStack(e))
		return r
	}

	r, e := b.digestLookupStore.Get(digest)
	if _, notFound := e.(*bitsgo.NotFoundError); notFound {
		return nil
	}

	util.PanicOnError(errors.WithStack(e))
	return r
}

func (b *BitsImageManager) configMetadata(rootfsDigest string, dropletDigest string) []byte {
	config, e := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"user": "vcap",
		},
		"rootfs": map[string]interface{}{
			"type": "layers",
			"diff_ids": []string{
				rootfsDigest,
				dropletDigest,
			},
		},
	})
	util.PanicOnError(errors.WithStack(e))
	return config
}

func (b *BitsImageManager) DeleteArtifacts(dropletGUID, dropletHash string) error {
	var errs []string
	manifestList := b.GetManifestList(dropletGUID, dropletHash)
	if manifestList == nil {
		return nil
	}

	if len(manifestList.Manifests) != 1 {
		errs = append(errs, "Unexpected number of manifests specified in manifest index. Expected: 1, Actual: "+fmt.Sprintf("%v", len(manifestList.Manifests)))
	}

	manifestListJSON, e := json.Marshal(manifestList)
	if e != nil {
		errs = append(errs, "Could not marshal manifest index struct into JSON: "+e.Error())
	} else {
		manifestListDigest, _ := shaAndSize(bytes.NewReader(manifestListJSON))
		e = b.digestLookupStore.Delete(manifestListDigest)
		if e != nil {
			errs = append(errs, "Could not delete manifest index JSON file from digest lookup store: "+e.Error())
		}
	}

	manifest := b.GetManifest(dropletGUID, dropletHash)
	if manifest == nil {
		errs = append(errs, "Could not find OCI manifest with droplet GUID "+dropletGUID+" and droplet hash "+dropletHash)
	} else {
		manifestJSON, e := json.Marshal(manifest)
		if e != nil {
			errs = append(errs, "Could not marshal manifest struct into JSON: "+e.Error())
		} else {
			manifestDigest, _ := shaAndSize(bytes.NewReader(manifestJSON))

			e = b.digestLookupStore.Delete(manifestDigest)
			if e != nil {
				errs = append(errs, "Could not delete OCI manifest JSON file from digest lookup store: "+e.Error())
			}

			e = b.digestLookupStore.Delete(manifest.Config.Digest)
			if e != nil {
				errs = append(errs, "Could not delete OCI manifest config JSON file from digest lookup store: "+e.Error())
			}
			if len(manifest.Layers) != 2 {
				errs = append(errs, "Unexpected number of layers specified in OCI manifest. Expected: 2, Actual: "+fmt.Sprintf("%v", len(manifest.Layers)))
			} else {
				e = b.digestLookupStore.Delete(manifest.Layers[1].Digest)
				if e != nil {
					errs = append(errs, "Could not delete OCI droplet layer file from digest lookup store: "+e.Error())
				}
			}
		}
	}

	if len(errs) != 0 {
		return errors.New("Unexpected errors while trying to delete OCI image artifacts: " + strings.Join(errs, ", "))
	}
	return nil
}
