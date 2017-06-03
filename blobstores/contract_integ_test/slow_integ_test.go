package main_test

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/petergtz/bitsgo/blobstores/azure"
	"github.com/petergtz/bitsgo/blobstores/gcp"
	"github.com/petergtz/bitsgo/blobstores/s3"
	"github.com/petergtz/bitsgo/config"
)

var _ = XDescribe("Non-local blobstores SLOW TESTS", func() {

	BeforeSuite(func() {
		rand.Seed(time.Now().Unix())
	})

	var (
		filepath  string
		blobstore blobstore
	)

	slowTests := func() {

		It("can upload a 500MB file", func() {
			filename, expectedMd5sum := createTempFileWithRandomContent(500 << 20)
			defer os.Remove(filename)

			file, e := os.Open(filename)
			Expect(e).NotTo(HaveOccurred())
			defer file.Close()

			e = blobstore.Put(filepath, file)
			Expect(e).NotTo(HaveOccurred())
			defer blobstore.Delete(filepath)

			reader, e := blobstore.Get(filepath)
			Expect(e).NotTo(HaveOccurred())
			defer reader.Close()

			Expect(md5sum(reader)).To(Equal(expectedMd5sum))
		})

		It("Can delete a dir with many files in it", func() {
			numManyFiles := 10000
			dirname := strconv.Itoa(rand.Int())
			randomFilenames := generateRandomFilenames(numManyFiles, dirname)

			By("Uploading files...")
			uploadFiles(blobstore, randomFilenames)
			By("Files uploaded.")

			By("Deleting dir...")
			Expect(blobstore.DeleteDir(dirname)).To(Succeed())
			By("Dir deleted.")

			time.Sleep(3 * time.Second) // Just help eventual consistency a bit

			By("Checking existence...")
			assertFilesDoNotExist(blobstore, randomFilenames)
			By("Existence checked.")
		})
	}

	var configFileContent []byte

	BeforeEach(func() {
		filename := os.Getenv("CONFIG")
		if filename == "" {
			fmt.Println("No $CONFIG set. Defaulting to integration_test_config.yml")
			filename = "integration_test_config.yml"
		}
		file, e := os.Open(filename)
		Expect(e).NotTo(HaveOccurred())
		defer file.Close()
		configFileContent, e = ioutil.ReadAll(file)
		Expect(e).NotTo(HaveOccurred())

		filepath = fmt.Sprintf("testfile-%v", time.Now())
	})

	Context("S3", func() {
		var s3Config config.S3BlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &s3Config)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = s3.NewBlobstore(s3Config) })

		slowTests()
	})

	Context("GCP", func() {
		var gcpConfig config.GCPBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &gcpConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = gcp.NewBlobstore(gcpConfig) })

		slowTests()
	})

	Context("azure", func() {
		var azureConfig config.AzureBlobstoreConfig

		BeforeEach(func() { Expect(yaml.Unmarshal(configFileContent, &azureConfig)).To(Succeed()) })
		JustBeforeEach(func() { blobstore = azure.NewBlobstore(azureConfig) })

		slowTests()
	})
})

func createTempFileWithRandomContent(size int64) (filename string, md5sum []byte) {
	file, e := ioutil.TempFile("", "")
	Expect(e).NotTo(HaveOccurred())
	defer file.Close()

	md5Expected := md5.New()
	_, e = io.CopyN(io.MultiWriter(file, md5Expected), rand.New(rand.NewSource(time.Now().Unix())), size)
	Expect(e).NotTo(HaveOccurred())

	return file.Name(), md5Expected.Sum(nil)
}

func md5sum(reader io.Reader) []byte {
	md5Actual := md5.New()
	_, e := io.Copy(md5Actual, reader)
	Expect(e).NotTo(HaveOccurred())
	return md5Actual.Sum((nil))
}

func generateRandomFilenames(numFilenames int, dirname string) []string {
	filenames := make([]string, numFilenames)
	for i := 0; i < numFilenames; i++ {
		filenames[i] = dirname + "/" + strconv.Itoa(rand.Int())
	}
	return filenames
}

func uploadFiles(blobstore blobstore, filenames []string) {
	numWorkers := 150
	assertionErrors := make(chan interface{}, 100)
	filenamesChannel := make(chan interface{}, 100)
	setUpWorkers(numWorkers, assertionErrors, filenamesChannel, func(unit interface{}) {
		filename := unit.(string)
		Expect(blobstore.Exists(filename)).To(BeFalse())
		Expect(blobstore.Put(filename, strings.NewReader("X"))).To(Succeed())
		Expect(blobstore.Exists(filename)).To(BeTrue())
	})

	go feedFilenamesInto(filenamesChannel, filenames)
	consumeErrors(len(filenames), assertionErrors)
}

func assertFilesDoNotExist(blobstore blobstore, filenames []string) {
	numWorkers := 150
	assertionErrors := make(chan interface{}, 100)
	filenamesChannel := make(chan interface{}, 100)
	setUpWorkers(numWorkers, assertionErrors, filenamesChannel, func(unit interface{}) {
		filename := unit.(string)
		Expect(blobstore.Exists(filename)).To(BeFalse())
	})

	go feedFilenamesInto(filenamesChannel, filenames)
	consumeErrors(len(filenames), assertionErrors)
}

func setUpWorkers(numWorkers int, errors chan<- interface{}, input <-chan interface{}, runTask func(unit interface{})) {
	for i := 0; i < numWorkers; i++ {
		go runWorker(errors, input, func(unit interface{}) {
			runTask(unit)
		})
	}
}

func runWorker(errors chan<- interface{}, input <-chan interface{}, runTask func(unit interface{})) {
	defer recoverIntoChannel(errors)
	for unit := range input {
		runTask(unit)
		errors <- nil
	}
}

func recoverIntoChannel(errors chan<- interface{}) {
	e := recover()
	if e != nil {
		errors <- e
	}
}

func feedFilenamesInto(channel chan interface{}, filenames []string) {
	for _, filename := range filenames {
		channel <- filename
	}
	close(channel)
}

func consumeErrors(numErrors int, errors chan interface{}) {
	for i := 0; i < numErrors; i++ {
		e := <-errors
		if e != nil {
			panic(e)
		}
	}
}
