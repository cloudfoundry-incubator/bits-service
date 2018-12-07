package oci_registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/cloudfoundry-incubator/bits-service"

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
	manifest := m.ImageManager.GetManifest(strings.TrimPrefix(mux.Vars(r)["name"], "cloudfoundry/"), mux.Vars(r)["tag"])

	if manifest == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Add("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
	w.Write(manifest)
}

func (m *ImageHandler) ServeLayer(w http.ResponseWriter, r *http.Request) {
	layer := m.ImageManager.GetLayer(mux.Vars(r)["name"], mux.Vars(r)["digest"])

	if layer == nil {
		http.NotFound(w, r)
		return
	}

	_, e := io.Copy(w, layer)

	util.PanicOnError(errors.WithStack(e))
}

type BitsImageManager struct {
	rootFSBlobstore   bitsgo.NoRedirectBlobstore
	dropletBlobstore  bitsgo.NoRedirectBlobstore
	digestLookupStore bitsgo.NoRedirectBlobstore
	rootfsSize        int64
	rootfsDigest      string
}

func NewBitsImageManager(
	rootFSBlobstore bitsgo.NoRedirectBlobstore,
	dropletBlobstore bitsgo.NoRedirectBlobstore,
	digestLookupStore bitsgo.NoRedirectBlobstore) *BitsImageManager {

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

func (b *BitsImageManager) GetManifest(dropletGUID string, dropletHash string) []byte {
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

	manifestJson, e := json.Marshal(docker.Manifest{
		MediaType:     mediatype.DistributionManifestJson,
		SchemaVersion: 2,
		Config: docker.Content{
			MediaType: mediatype.ContainerImageJson,
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
	})
	util.PanicOnError(errors.WithStack(e))

	e = b.digestLookupStore.Put(configDigest, bytes.NewReader(configJSON))
	util.PanicOnError(errors.WithStack(e))

	return manifestJson
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
func (b *BitsImageManager) GetLayer(name string, digest string) io.ReadCloser {
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
	// TODO: ns, tzip why is this necessary?
	// TDOO: ns, tzip how to handle this?
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
