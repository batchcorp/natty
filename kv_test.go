// NOTE: These tests require NATS to be available on "localhost"
package natty

import (
	"context"
	"math/rand"
	"time"

	"github.com/nats-io/nats.go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	uuid "github.com/satori/go.uuid"
)

var _ = Describe("KV", func() {
	var (
		cfg *Config
		n   *Natty
	)

	BeforeEach(func() {
		var err error

		cfg = NewConfig()

		n, err = New(cfg)

		Expect(err).To(BeNil())
		Expect(n).NotTo(BeNil())
	})

	Describe("Get", func() {
		It("should return the value for a key", func() {
			bucket, key, value := NewKVSet()

			kv, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket:      bucket,
				Description: "created during kv test",
			})

			Expect(err).To(BeNil())

			Expect(kv).NotTo(BeNil())

			_, putErr := kv.Put(key, []byte(value))
			Expect(putErr).To(BeNil())

			data, err := n.Get(nil, bucket, key)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal([]byte(value)))
		})

		It("should not auto-create a bucket", func() {
			data, err := n.Get(nil, "non-existent-bucket", "non-existent-key")
			Expect(err).To(Equal(nats.ErrKeyNotFound))
			Expect(data).To(BeNil())

			kv, err := n.js.KeyValue("non-existent-bucket")
			Expect(err).To(Equal(nats.ErrBucketNotFound))
			Expect(kv).To(BeNil())
		})
	})

	Describe("Create", func() {
		It("should auto-create bucket + create kv entry", func() {
			bucket, key, value := NewKVSet()

			putErr := n.Create(nil, bucket, key, value)
			Expect(putErr).ToNot(HaveOccurred())

			// Bucket should've been created
			kv, err := n.js.KeyValue(bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(kv).NotTo(BeNil())

			// K/V should've been created
			kve, err := kv.Get(key)
			Expect(err).ToNot(HaveOccurred())
			Expect(kve).NotTo(BeNil())

			// Values should match
			Expect(kve.Value()).To(Equal(value))
		})

		It("should work if bucket already exists", func() {
			bucket, key, value := NewKVSet()
			ttl := 10 * time.Second

			// Pre-create bucket
			_, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket: bucket,
				TTL:    ttl,
			})

			Expect(err).ToNot(HaveOccurred())

			// Verify that bucket exists
			kv, err := n.js.KeyValue(bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(kv).NotTo(BeNil())

			// Create entry
			err = n.Create(nil, bucket, key, value)
			Expect(err).ToNot(HaveOccurred())

			// Did the entry get created?
			kve, err := kv.Get(key)
			Expect(err).ToNot(HaveOccurred())
			Expect(kve).NotTo(BeNil())

			// Values should match
			Expect(kve.Value()).To(Equal(value))
		})

		It("should error if key already exists in bucket", func() {
			bucket, key, value := NewKVSet()
			ttl := 10 * time.Second

			// Pre-create bucket
			kv, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket: bucket,
				TTL:    ttl,
			})

			Expect(err).ToNot(HaveOccurred())

			// Pre-add key
			_, err = kv.Create(key, value)
			Expect(err).ToNot(HaveOccurred())

			// Attempt to create for same key should error
			err = n.Create(nil, bucket, key, value)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("wrong last sequence"))
		})

		It("should use TTL", func() {
			bucket, key, value := NewKVSet()
			ttl := 10 * time.Second

			_, err := n.js.KeyValue(bucket)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(nats.ErrBucketNotFound))

			err = n.Create(nil, bucket, key, value, ttl)
			Expect(err).ToNot(HaveOccurred())

			kv, err := n.js.KeyValue(bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(kv).ToNot(BeNil())

			status, err := kv.Status()
			Expect(err).ToNot(HaveOccurred())

			Expect(status.TTL()).To(Equal(ttl))
		})
	})

	Describe("Put", func() {
		It("should set the value for a key (and auto-create the bucket)", func() {
			bucket, key, value := NewKVSet()

			putErr := n.Put(nil, bucket, key, value)
			Expect(putErr).ToNot(HaveOccurred())

			// Bucket should've been created
			kv, err := n.js.KeyValue(bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(kv).NotTo(BeNil())

			// K/V should've been created
			kve, err := kv.Get(key)
			Expect(err).ToNot(HaveOccurred())
			Expect(kve).NotTo(BeNil())

			// Values should match
			Expect(kve.Value()).To(Equal(value))
		})

		It("a key with a TTL will get auto expired", func() {
			bucket, key, value := NewKVSet()

			putErr := n.Put(nil, bucket, key, value, 1*time.Second)
			Expect(putErr).ToNot(HaveOccurred())

			// Bucket should've been created
			kv, err := n.js.KeyValue(bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(kv).NotTo(BeNil())

			// Wait a couple sec
			time.Sleep(2 * time.Second)

			// K/V should no longer be there
			kve, err := kv.Get(key)
			Expect(err).To(HaveOccurred())
			Expect(kve).To(BeNil())
			Expect(err).To(Equal(nats.ErrKeyNotFound))

		})
	})

	Describe("Delete", func() {
		It("should delete the value for a key", func() {
			bucket, key, value := NewKVSet()

			// Create a bucket + key
			kv, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket:      bucket,
				Description: "created during kv test",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(kv).ToNot(BeNil())

			_, putErr := kv.Put(key, value)
			Expect(putErr).ToNot(HaveOccurred())

			// Try to delete it
			delErr := n.Delete(nil, bucket, key)
			Expect(delErr).ToNot(HaveOccurred())

			// Check via js context that it's gone
			_, getErr := kv.Get(key)
			Expect(getErr).To(Equal(nats.ErrKeyNotFound))
		})
	})

	Describe("Keys", func() {
		It("should return all keys in bucket", func() {
			// Create bucket, add a bunch of keys into it
			bucket, _, _ := NewKVSet()

			kv, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket:      bucket,
				Description: "tmp bucket for testing Keys()",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(kv).ToNot(BeNil())

			numKeys := rand.Intn(20) + 1 // + 1 to avoid 0

			for i := 0; i < numKeys; i++ {
				_, putErr := kv.Put(uuid.NewV4().String(), []byte("test"))
				Expect(putErr).ToNot(HaveOccurred())
			}

			keys, err := n.Keys(context.Background(), bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(keys)).To(Equal(numKeys))
		})

		It("should return emtpy slice if no keys in bucket", func() {
			// Create bucket, add a bunch of keys into it
			bucket, _, _ := NewKVSet()

			kv, err := n.js.CreateKeyValue(&nats.KeyValueConfig{
				Bucket:      bucket,
				Description: "tmp bucket for testing Keys()",
			})

			Expect(err).ToNot(HaveOccurred())
			Expect(kv).ToNot(BeNil())

			keys, err := n.Keys(context.Background(), bucket)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(keys)).To(Equal(0))
		})

		It("should error if bucket does not exist", func() {
			keys, err := n.Keys(context.Background(), uuid.NewV4().String())
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(nats.ErrBucketNotFound))
			Expect(keys).To(BeNil())
		})
	})
})

func NewKVSet() (bucket string, key string, value []byte) {
	bucket = uuid.NewV4().String()
	key = uuid.NewV4().String()
	value = []byte(uuid.NewV4().String())

	testBuckets = append(testBuckets, bucket)

	return
}
