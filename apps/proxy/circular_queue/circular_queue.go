// CircularQueue implements a circular queue of RTCM messages.
//
// NewCircularQueue(n) creates a circular queue that holds up to n messages.
// The queue contains a pointer to a mutex so always use this function to
// create a queue.
//
// Add(message) adds a message to the queue.  If the queue is already full, it
// removes the oldest message to make way for the new one.
//
// GetMessages() gets the messages in the circular queue as a slice,
// in the order in which they were added.
package circularQueue

import (
	"sort"
	"sync"

	rtcm "github.com/goblimey/go-ntrip/rtcm/handler"
)

// CircularQueue holds a limited number of RTCM messages.  If a
// message is added and the buffer is already full, the oldest
// message is removed to make way for the new one.  The buffer
// is safe against asynchronous access.
type CircularQueue struct {
	// MaxItems is the maximum number of items in the circular queue.
	MaxItems int
	// Items is a map containing the messages each with a unique index.
	Items map[int]rtcm.Message
	// NextIndex is the next unique index, assigned when a message is added.
	NextIndex int

	// The mutex controls asynchronous access to the circular queue.
	// See https://go.dev/blog/maps
	*sync.RWMutex
}

// NewCircularQueue creates a new circular queue.
func NewCircularQueue(max int) *CircularQueue {
	items := make(map[int]rtcm.Message, max)
	var mu sync.RWMutex
	q := CircularQueue{MaxItems: max, Items: items, NextIndex: 0, RWMutex: &mu}

	return &q
}

// Add adds a new message to the queue, removing the oldest if necessary.
func (cb *CircularQueue) Add(message rtcm.Message) {
	// Write lock.
	cb.Lock()
	defer cb.Unlock()

	// If the map is already full, remove items until there is space.
	if len(cb.Items) >= cb.MaxItems {
		keys := cb.getKeysInAscendingOrder()
		// (We should only have to remove one message, but keep checking,
		// just in case)
		for _, key := range keys {
			if len(cb.Items) >= cb.MaxItems {
				delete(cb.Items, key)
			}
		}
	}

	// Add the message to the buffer.
	cb.Items[cb.NextIndex] = message
	// Update the unique index.  It's an int and may be only 32 bits
	// so if this is called many times the index will eventually overflow.
	cb.NextIndex++
}

// GetMessages gets the items in the circular queue as a slice,
// in ascending order of key, ie in the order that they were added.
func (cb *CircularQueue) GetMessages() []rtcm.Message {
	// Read lock.
	cb.RLock()
	defer cb.RUnlock()

	keys := cb.getKeysInAscendingOrder()
	result := make([]rtcm.Message, 0)
	for _, k := range keys {
		item, ok := cb.Items[k]
		// There should always be an item with that key but never say never ...
		if ok {
			result = append(result, item)
		}
	}

	return result
}

// getKeysInAscendingOrder gets the keys in ascending order.
func (cb *CircularQueue) getKeysInAscendingOrder() []int {

	// The map keys are not presented in order by default -
	// See https://go.dev/blog/maps.

	var keys []int
	for k := range cb.Items {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	return keys
}
