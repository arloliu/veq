package eapsml

// Transaction is a primary and its reply.
// Primary is nil for a reply whose primary is not in the log.
type Transaction struct {
	Primary *Message
	Reply   *Message
}

type txKey struct {
	dir      Direction // direction of the primary
	stream   uint8
	function uint8 // function of the primary
	tid      uint32
}

// Correlator pairs replies with primaries by direction, stream, function and the EAP's TID.
// A primary left unreplied means only that the log lacks the reply,
// not that none was sent.
type Correlator struct {
	pending map[txKey]*Message
}

// NewCorrelator returns an empty Correlator.
func NewCorrelator() *Correlator {
	return &Correlator{pending: map[txKey]*Message{}}
}

// Add records m.
// It returns the transaction when m is a reply,
// and nil when m is a primary or its direction is unknown.
// A primary that reuses the key of an unreplied one replaces it.
func (c *Correlator) Add(m *Message) *Transaction {
	if m.Dir == DirUnknown {
		return nil
	}
	if m.Function%2 == 1 {
		if m.WBit {
			c.pending[txKey{m.Dir, m.Stream, m.Function, m.TID}] = m
		}

		return nil
	}
	k := txKey{opposite(m.Dir), m.Stream, m.Function - 1, m.TID}
	p := c.pending[k]
	delete(c.pending, k)

	return &Transaction{Primary: p, Reply: m}
}

// Unreplied returns the primaries with the W-bit for which no reply has been added.
func (c *Correlator) Unreplied() []*Message {
	out := make([]*Message, 0, len(c.pending))
	for _, m := range c.pending {
		out = append(out, m)
	}

	return out
}

func opposite(d Direction) Direction {
	switch d {
	case EquipToHost:
		return HostToEquip
	case HostToEquip:
		return EquipToHost
	default:
		return DirUnknown
	}
}
