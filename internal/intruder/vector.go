package intruder

import "fmt"

// VectorIterator yields one per-request payload vector at a time. The
// vector's length is the attack's position count: 1 for Sniper, N for
// Pitchfork / ClusterBomb. Iterators are not safe for concurrent use;
// the runner pulls from a single instance on the dispatcher goroutine.
type VectorIterator interface {
	Next() ([]string, bool)
}

// NewVectorIterator constructs the iterator that the runner dispatches
// from. The mode dictates how the per-position PayloadConfigs combine:
//
//   - Sniper: cfgs must have exactly one entry. The vector is always
//     length 1.
//   - Pitchfork: cfgs has N>=2 entries; positions iterate in parallel
//     (zip) and the run ends when any list exhausts.
//   - ClusterBomb: cfgs has N>=2 entries; the iterator enumerates the
//     Cartesian product. To keep this practical without infinite
//     PayloadIterator rewinds, every position is materialised into a
//     []string up front; the runner's MaxRequests still bounds the
//     dispatch count.
func NewVectorIterator(mode AttackMode, cfgs []PayloadConfig) (VectorIterator, error) {
	switch mode {
	case Sniper:
		if len(cfgs) != 1 {
			return nil, fmt.Errorf("sniper: expected 1 payload list, got %d", len(cfgs))
		}
		it, err := NewIterator(cfgs[0])
		if err != nil {
			return nil, err
		}
		return &sniperVecIter{inner: it}, nil

	case Pitchfork:
		if len(cfgs) < 2 {
			return nil, fmt.Errorf("pitchfork: requires at least 2 payload lists, got %d", len(cfgs))
		}
		inners := make([]PayloadIterator, len(cfgs))
		for i, c := range cfgs {
			it, err := NewIterator(c)
			if err != nil {
				return nil, fmt.Errorf("pitchfork: payload %d: %w", i+1, err)
			}
			inners[i] = it
		}
		return &pitchforkVecIter{inners: inners}, nil

	case ClusterBomb:
		if len(cfgs) < 2 {
			return nil, fmt.Errorf("cluster bomb: requires at least 2 payload lists, got %d", len(cfgs))
		}
		// Materialise every position into a slice. PayloadIterator is
		// one-pass (Brute and CaseToggle have no rewind), and the
		// Cartesian odometer has to revisit each list many times.
		lists := make([][]string, len(cfgs))
		for i, c := range cfgs {
			it, err := NewIterator(c)
			if err != nil {
				return nil, fmt.Errorf("cluster bomb: payload %d: %w", i+1, err)
			}
			for {
				v, ok := it.Next()
				if !ok {
					break
				}
				lists[i] = append(lists[i], v)
			}
			if len(lists[i]) == 0 {
				return nil, fmt.Errorf("cluster bomb: payload %d produced 0 values", i+1)
			}
		}
		return &clusterBombVecIter{lists: lists, indices: make([]int, len(lists))}, nil
	}
	return nil, fmt.Errorf("unknown attack mode: %d", mode)
}

// sniperVecIter wraps a single PayloadIterator and yields singleton
// vectors, preserving the v1.2.x behaviour.
type sniperVecIter struct {
	inner PayloadIterator
}

func (s *sniperVecIter) Next() ([]string, bool) {
	v, ok := s.inner.Next()
	if !ok {
		return nil, false
	}
	return []string{v}, true
}

// pitchforkVecIter pulls one value from each inner iterator per call;
// iteration ends as soon as any inner is exhausted (zip-stops-shortest).
type pitchforkVecIter struct {
	inners []PayloadIterator
}

func (p *pitchforkVecIter) Next() ([]string, bool) {
	out := make([]string, len(p.inners))
	for i, it := range p.inners {
		v, ok := it.Next()
		if !ok {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// clusterBombVecIter enumerates the Cartesian product of materialised
// lists using an odometer: rightmost position increments first, with
// carry propagating left. When the leftmost overflows, iteration ends.
type clusterBombVecIter struct {
	lists   [][]string
	indices []int
	done    bool
}

func (c *clusterBombVecIter) Next() ([]string, bool) {
	if c.done {
		return nil, false
	}
	out := make([]string, len(c.lists))
	for i, idx := range c.indices {
		out[i] = c.lists[i][idx]
	}
	// Advance the odometer for next call.
	for j := len(c.indices) - 1; j >= 0; j-- {
		c.indices[j]++
		if c.indices[j] < len(c.lists[j]) {
			return out, true
		}
		c.indices[j] = 0
	}
	// Carried past the leftmost — this was the final combination.
	c.done = true
	return out, true
}
