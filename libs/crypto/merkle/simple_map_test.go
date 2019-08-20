package merkle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
)

type strHasher string

func (str strHasher) Hash() []byte {
	hw := sha3.NewLegacyKeccak256()
	hw.Write([]byte(str))
	return hw.Sum(nil)
}

func TestSimpleMap(t *testing.T) {
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "361b09087641a98ad1106fd81366e829d3a9787dcaed6b3071acbd5b503b2ea2", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value2"))
		assert.Equal(t, "cfa4a97f95e5cb29124dc6ff7d154ea22be891df6f9dbb8aad96b051e2727e6e", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		assert.Equal(t, "9e6a42d9f1b6157f6872cb9f4faf6b2fb2ffcb51d6ccc7ad6688c83ec2f5b836", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "9e6a42d9f1b6157f6872cb9f4faf6b2fb2ffcb51d6ccc7ad6688c83ec2f5b836", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "ab223c789f592199d46f597f18e9addaa8cabcc5b43bcf477a6ac031e039f8b1", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "ab223c789f592199d46f597f18e9addaa8cabcc5b43bcf477a6ac031e039f8b1", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
