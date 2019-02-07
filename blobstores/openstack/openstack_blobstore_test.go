package openstack_test

import (
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/cloudfoundry-incubator/bits-service/blobstores/openstack"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const numWorkers = 1000

var _ = Describe("DeleteInParallel", func() {
	Context("names is empty", func() {
		It("doesn't call the deletionFunc", func() {
			errs := DeleteInParallel([]string{}, numWorkers, func(string) error {
				defer GinkgoRecover()

				Fail("This function should not be called for an empty names slice")
				return nil
			})
			Expect(errs).To(BeEmpty())
		})
	})

	Context("names contains one element", func() {
		It("calls the deletionFunc once and returns no error", func() {
			var m sync.Mutex
			namesDeleted := make(map[string]bool)
			errs := DeleteInParallel([]string{"foo"}, numWorkers, func(name string) error {
				m.Lock()
				defer m.Unlock()
				namesDeleted[name] = true
				return nil
			})
			Expect(errs).To(BeEmpty())
			Expect(namesDeleted).To(SatisfyAll(
				HaveLen(1),
				HaveKey("foo")))
		})

		Context("deletionFunc returns an error", func() {
			It("returns the error as a result", func() {
				errs := DeleteInParallel([]string{"foo"}, numWorkers, func(string) error {
					time.Sleep(10 * time.Millisecond)
					return errors.New("some error")
				})
				Expect(errs).To(ConsistOf(MatchError("some error")))
			})
		})
	})

	Context("names contains many elements", func() {
		const numNames = 10000
		var names []string

		BeforeEach(func() {
			names = make([]string, numNames)
			for i := 0; i < numNames; i++ {
				names[i] = fmt.Sprintf("%v", i)
			}
		})

		Context("names contains many elements where each item takes 10ms to delete ", func() {
			It("calls the deletionFunc for every item and finishes within 2s", func(done Done) {
				var m sync.Mutex
				namesDeleted := make(map[string]bool)

				errs := DeleteInParallel(names, numWorkers, func(name string) error {
					time.Sleep(10 * time.Millisecond)
					m.Lock()
					defer m.Unlock()
					namesDeleted[name] = true
					return nil
				})

				Expect(errs).To(BeEmpty())
				Expect(namesDeleted).To(HaveLen(numNames))
				for _, name := range names {
					if _, exists := namesDeleted[name]; !exists {
						Fail("Name not deleted :" + name)
					}
				}

				close(done)
			}, 2.0)
		})

		Context("names contains many elements where all items return an erro after 10ms", func() {
			It("calls the deletionFunc for every item and finishes within 2s", func(done Done) {
				errs := DeleteInParallel(names, numWorkers, func(name string) error {
					time.Sleep(10 * time.Millisecond)
					return errors.New("some error")
				})
				Expect(len(errs)).To(Equal(numNames))

				close(done)
			}, 2.0)

		})
	})

})
