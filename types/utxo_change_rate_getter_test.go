package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUTXOChangeRateFromUint8(t *testing.T) {
	res, err := UTXOChangeRateFromUint8(uint8(18 + 8))
	assert.Nil(t, err)
	assert.Equal(t, int64(1e18), res)
	res, err = UTXOChangeRateFromUint8(uint8(18))
	assert.Nil(t, err)
	assert.Equal(t, int64(1e10), res)
	res, err = UTXOChangeRateFromUint8(uint8(8))
	assert.Nil(t, err)
	assert.Equal(t, int64(1e0), res)
	res, err = UTXOChangeRateFromUint8(uint8(0))
	assert.Nil(t, err)
	assert.Equal(t, int64(1e0), res)
	res, err = UTXOChangeRateFromUint8(uint8(18 + 9))
	assert.Equal(t, ErrUTXOChangeRateTooLarge, err)
	assert.Equal(t, int64(0), res)
}
