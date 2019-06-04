package testutil

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func CreateZip(contents map[string]string) *bytes.Buffer {
	var result bytes.Buffer
	zipWriter := zip.NewWriter(&result)
	defer zipWriter.Close()
	for filename, fileContents := range contents {
		entryWriter, e := zipWriter.Create(filename)
		Expect(e).NotTo(HaveOccurred())
		entryWriter.Write([]byte(fileContents))

	}
	e := zipWriter.Close()
	Expect(e).NotTo(HaveOccurred())
	return &result
}

func CreateGZip(contents map[string]string) *bytes.Buffer {
	var result bytes.Buffer
	zipWriter := gzip.NewWriter(&result)
	t := tar.NewWriter(zipWriter)

	defer zipWriter.Close()
	for filename, fileContents := range contents {
		e := t.WriteHeader(&tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(fileContents)),
		})
		Expect(e).NotTo(HaveOccurred())
		t.Write([]byte(fileContents))

	}
	e := zipWriter.Close()
	Expect(e).NotTo(HaveOccurred())
	return &result
}

func VerifyZipFileEntry(reader *zip.Reader, expectedFilename string, expectedContent string) {
	var foundEntries []string
	for _, entry := range reader.File {
		if entry.Name == expectedFilename {
			content, e := entry.Open()
			Expect(e).NotTo(HaveOccurred())
			Expect(ioutil.ReadAll(content)).To(MatchRegexp(expectedContent), "for filename "+expectedFilename)
			return
		}
		foundEntries = append(foundEntries, entry.Name)
	}
	Fail("Did not find entry with name " + expectedFilename + ". Found only: " + strings.Join(foundEntries, ", "))
}
