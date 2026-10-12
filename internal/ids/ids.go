// Package ids generates record identifiers. Production uses time-ordered random
// identifiers; tests use a sequential generator for deterministic output.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

type Generator interface {
	New(prefix string) string
}

// TimeRandom yields "<prefix>_<unix-ms base36><6 random bytes hex>".
type TimeRandom struct{}

func (TimeRandom) New(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s%s", prefix, strconv.FormatInt(time.Now().UnixMilli(), 36), hex.EncodeToString(b))
}

// Sequential yields "<prefix>_0001", "<prefix>_0002", ... per prefix.
type Sequential struct {
	mu sync.Mutex
	n  map[string]int
}

func (s *Sequential) New(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.n == nil {
		s.n = map[string]int{}
	}
	s.n[prefix]++
	return fmt.Sprintf("%s_%04d", prefix, s.n[prefix])
}
