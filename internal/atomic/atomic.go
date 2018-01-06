package atomic

import (
	"sync/atomic"
)

// noCopy may be embedded into structs which must not be copied
// after the first use.
//
// See https://github.com/golang/go/issues/8005#issuecomment-190753527
// for details.
type noCopy struct{}

// Lock is a no-op used by -copylocks checker from `go vet`.
func (*noCopy) Lock() {}

// Bool is a wrapper around uint32 for usage as a boolean value with
// atomic access.
type Bool struct {
	_noCopy noCopy
	value   uint32
}

// IsSet returns whether the current boolean value is true
func (b *Bool) IsSet() bool {
	return atomic.LoadUint32(&b.value) > 0
}

// Set sets the value of the bool regardless of the previous value
func (b *Bool) Set(value bool) {
	if value {
		atomic.StoreUint32(&b.value, 1)
	} else {
		atomic.StoreUint32(&b.value, 0)
	}
}

// TrySet sets the value of the bool and returns whether the value has changed
func (b *Bool) TrySet(value bool) bool {
	if value {
		return atomic.SwapUint32(&b.value, 1) == 0
	}
	return atomic.SwapUint32(&b.value, 0) > 0
}
