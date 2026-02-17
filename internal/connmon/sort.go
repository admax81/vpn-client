package connmon

import "sort"

// SortConnectionsByState sorts connections: ESTABLISHED first, then SYN_SENT, etc.
func SortConnectionsByState(conns []Connection) {
	sort.Slice(conns, func(i, j int) bool {
		// ESTABLISHED first, then SYN_SENT, then others
		pi := statePriority(conns[i].State)
		pj := statePriority(conns[j].State)
		if pi != pj {
			return pi < pj
		}
		return conns[i].RemoteAddr.Addr().Less(conns[j].RemoteAddr.Addr())
	})
}

func statePriority(s ConnState) int {
	switch s {
	case StateEstablished:
		return 0
	case StateSynSent:
		return 1
	case StateTimeWait:
		return 2
	case StateCloseWait:
		return 3
	default:
		return 4
	}
}
