package handler

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"polaris-platform/internal/model"
)

const (
	maxProbeFrameBytes = 4 * 1024 * 1024
	probeBatchSize     = 400
	probeDialTimeout   = 800 * time.Millisecond
)

func appendProtoVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

func consumeProtoVarint(src []byte) (uint64, int, error) {
	var v uint64
	for i := 0; i < 10 && i < len(src); i++ {
		b := src[i]
		v |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return v, i + 1, nil
		}
	}
	return 0, 0, errors.New("truncated or overflowing varint")
}

func appendProtoLenField(dst []byte, fieldNum int, payload []byte) []byte {
	dst = appendProtoVarint(dst, (uint64(fieldNum)<<3)|2)
	dst = appendProtoVarint(dst, uint64(len(payload)))
	return append(dst, payload...)
}

func encodeOnlineRequest(selfID string, peers []string) []byte {
	var req []byte
	if selfID != "" {
		req = appendProtoLenField(req, 1, []byte(selfID))
	}
	for _, peer := range peers {
		if peer == "" {
			continue
		}
		req = appendProtoLenField(req, 2, []byte(peer))
	}
	return appendProtoLenField(nil, 23, req)
}

func decodeOnlineResponse(frame []byte) (states []byte, isOnlineResp bool, err error) {
	pos := 0
	for pos < len(frame) {
		tag, n, err := consumeProtoVarint(frame[pos:])
		if err != nil {
			return nil, false, err
		}
		pos += n
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch wireType {
		case 0:
			_, vn, err := consumeProtoVarint(frame[pos:])
			if err != nil {
				return nil, false, err
			}
			pos += vn
		case 1:
			if pos+8 > len(frame) {
				return nil, false, errors.New("truncated fixed64")
			}
			pos += 8
		case 2:
			ln, lnBytes, err := consumeProtoVarint(frame[pos:])
			if err != nil {
				return nil, false, err
			}
			pos += lnBytes
			if ln > uint64(len(frame)-pos) {
				return nil, false, errors.New("truncated length-delimited field")
			}
			val := frame[pos : pos+int(ln)]
			pos += int(ln)
			if fieldNum == 24 {
				states, err = decodeOnlineResponseBody(val)
				return states, true, err
			}
		case 5:
			if pos+4 > len(frame) {
				return nil, false, errors.New("truncated fixed32")
			}
			pos += 4
		default:
			return nil, false, fmt.Errorf("unsupported wire type %d", wireType)
		}
	}
	return nil, false, nil
}

func decodeOnlineResponseBody(body []byte) ([]byte, error) {
	pos := 0
	for pos < len(body) {
		tag, n, err := consumeProtoVarint(body[pos:])
		if err != nil {
			return nil, err
		}
		pos += n
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)
		switch wireType {
		case 0:
			_, vn, err := consumeProtoVarint(body[pos:])
			if err != nil {
				return nil, err
			}
			pos += vn
		case 1:
			if pos+8 > len(body) {
				return nil, errors.New("truncated fixed64")
			}
			pos += 8
		case 2:
			ln, lnBytes, err := consumeProtoVarint(body[pos:])
			if err != nil {
				return nil, err
			}
			pos += lnBytes
			if ln > uint64(len(body)-pos) {
				return nil, errors.New("truncated states bytes")
			}
			val := body[pos : pos+int(ln)]
			pos += int(ln)
			if fieldNum == 1 {
				out := make([]byte, len(val))
				copy(out, val)
				return out, nil
			}
		case 5:
			if pos+4 > len(body) {
				return nil, errors.New("truncated fixed32")
			}
			pos += 4
		default:
			return nil, fmt.Errorf("unsupported inner wire type %d", wireType)
		}
	}
	return nil, nil
}

func encodeBytesCodecFrame(payload []byte) ([]byte, error) {
	n := len(payload)
	if n > 0x3FFFFFFF {
		return nil, errors.New("frame payload too large")
	}
	var hdr []byte
	switch {
	case n <= 0x3F:
		hdr = []byte{byte(n << 2)}
	case n <= 0x3FFF:
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16((n<<2)|0x1))
		hdr = b[:]
	case n <= 0x3FFFFF:
		v := uint32((n << 2) | 0x2)
		hdr = []byte{byte(v), byte(v >> 8), byte(v >> 16)}
	default:
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], uint32((n<<2)|0x3))
		hdr = b[:]
	}
	out := make([]byte, len(hdr)+n)
	copy(out, hdr)
	copy(out[len(hdr):], payload)
	return out, nil
}

func readBytesCodecFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:1]); err != nil {
		return nil, err
	}
	headLen := int(hdr[0]&0x3) + 1
	if headLen > 1 {
		if _, err := io.ReadFull(r, hdr[1:headLen]); err != nil {
			return nil, err
		}
	}
	raw := uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16 | uint32(hdr[3])<<24
	n := int(raw >> 2)
	if n < 0 || n > maxProbeFrameBytes {
		return nil, fmt.Errorf("invalid frame length: %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func parseOnlineStatesBitmask(ids []string, states []byte) map[string]bool {
	res := make(map[string]bool, len(ids))
	for i, id := range ids {
		if i/8 >= len(states) {
			res[id] = false
			continue
		}
		bit := byte(0x80 >> uint(i%8))
		res[id] = (states[i/8] & bit) == bit
	}
	return res
}

func probeHbbsAddr(addr string, ids []string, timeout time.Duration) (map[string]bool, int64, error) {
	start := time.Now()
	if len(ids) == 0 {
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, 0, err
		}
		_ = conn.Close()
		return map[string]bool{}, time.Since(start).Milliseconds(), nil
	}

	out := make(map[string]bool, len(ids))
	for offset := 0; offset < len(ids); offset += probeBatchSize {
		end := offset + probeBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[offset:end]

		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err != nil {
			return nil, 0, err
		}
		_ = conn.SetDeadline(time.Now().Add(timeout))
		frame, err := encodeBytesCodecFrame(encodeOnlineRequest("polaris-platform", chunk))
		if err != nil {
			_ = conn.Close()
			return nil, 0, err
		}
		if _, err := conn.Write(frame); err != nil {
			_ = conn.Close()
			return nil, 0, err
		}

		var states []byte
		gotResp := false
		for attempt := 0; attempt < 4; attempt++ {
			respFrame, err := readBytesCodecFrame(conn)
			if err != nil {
				_ = conn.Close()
				return nil, 0, err
			}
			st, ok, err := decodeOnlineResponse(respFrame)
			if err != nil {
				_ = conn.Close()
				return nil, 0, err
			}
			if ok {
				states = st
				gotResp = true
				break
			}
		}
		_ = conn.Close()
		if !gotResp {
			return nil, 0, errors.New("no OnlineResponse received from hbbs")
		}
		for k, v := range parseOnlineStatesBitmask(chunk, states) {
			out[k] = v
		}
	}
	return out, time.Since(start).Milliseconds(), nil
}

func (h *Handler) probeAddrs() []string {
	if env := strings.TrimSpace(os.Getenv("POLARIS_HBBS_PROBE_ADDR")); env != "" {
		lower := strings.ToLower(env)
		if lower == "off" || lower == "false" || lower == "0" || lower == "none" {
			return nil
		}
		return []string{env}
	}
	if h.Cfg == nil || h.Cfg.RendezvousPort <= 1 {
		return nil
	}
	probePort := strconv.Itoa(h.Cfg.RendezvousPort - 1)
	addrs := []string{net.JoinHostPort("127.0.0.1", probePort)}
	if host := strings.TrimSpace(h.Cfg.ServerPublicHost); host != "" && host != "127.0.0.1" && host != "localhost" {
		addrs = append(addrs, net.JoinHostPort(host, probePort))
	}
	return addrs
}

func (h *Handler) probeLiveStates(ids []string) (map[string]bool, bool, int64) {
	addrs := h.probeAddrs()
	if h.hub != nil {
		if cached, ok := h.hub.preferredAddr(); ok {
			if states, ms, err := probeHbbsAddr(cached, ids, probeDialTimeout); err == nil {
				return states, true, ms
			}
			h.hub.clearPreferredAddr(cached)
		}
	}
	for _, addr := range addrs {
		if h.hub != nil && h.hub.isAddrCoolingDown(addr) {
			continue
		}
		if states, ms, err := probeHbbsAddr(addr, ids, probeDialTimeout); err == nil {
			if h.hub != nil {
				h.hub.setPreferredAddr(addr)
			}
			return states, true, ms
		}
		if h.hub != nil && !strings.HasPrefix(addr, "127.0.0.1:") {
			h.hub.markAddrFailed(addr)
		}
	}
	return nil, false, 0
}

func calcLastSeenSec(lastSeen time.Time) int64 {
	if lastSeen.IsZero() {
		return -1
	}
	sec := int64(time.Since(lastSeen).Seconds())
	if sec < 0 {
		return 0
	}
	return sec
}

func (h *Handler) applyDeviceLiveStatus(d *model.Device, hbbsStates map[string]bool, hbbsReachable bool) {
	d.LastSeenSec = calcLastSeenSec(d.LastSeenAt)
	hbOnline := h.isDeviceOnline(d.LastSeenAt)
	if hbbsReachable {
		v := hbbsStates[d.ID]
		d.HbbsOnline = &v
		if d.ClientRole == "console" {
			d.Online = v || hbOnline
		} else {
			d.Online = v
		}
		return
	}
	d.HbbsOnline = nil
	d.Online = hbOnline
}

// --- Live SSE Hub & Background Liveness Monitor ---

type liveSubscriber struct {
	id       uint64
	schoolID uint
	ch       chan string
}

type deviceSnapshot struct {
	online       bool
	hbbsOnline   int8 // -1 unknown, 0 false, 1 true
	activeConns  int
	lastSeenUnix int64
}

type liveHub struct {
	mu           sync.Mutex
	subs         map[uint64]*liveSubscriber
	nextID       uint64
	monitorOn    bool
	lastSnap     map[string]deviceSnapshot
	prefAddr     string
	prefAddrTime time.Time
	failedAddrs  map[string]time.Time
}

func newLiveHub() *liveHub {
	return &liveHub{
		subs:        make(map[uint64]*liveSubscriber),
		lastSnap:    make(map[string]deviceSnapshot),
		failedAddrs: make(map[string]time.Time),
	}
}

func (hub *liveHub) preferredAddr() (string, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.prefAddr == "" || time.Since(hub.prefAddrTime) > 60*time.Second {
		return "", false
	}
	return hub.prefAddr, true
}

func (hub *liveHub) setPreferredAddr(addr string) {
	hub.mu.Lock()
	hub.prefAddr = addr
	hub.prefAddrTime = time.Now()
	delete(hub.failedAddrs, addr)
	hub.mu.Unlock()
}

func (hub *liveHub) clearPreferredAddr(addr string) {
	hub.mu.Lock()
	if hub.prefAddr == addr {
		hub.prefAddr = ""
	}
	hub.mu.Unlock()
}

func (hub *liveHub) isAddrCoolingDown(addr string) bool {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	t, ok := hub.failedAddrs[addr]
	return ok && time.Since(t) < 15*time.Second
}

func (hub *liveHub) markAddrFailed(addr string) {
	hub.mu.Lock()
	hub.failedAddrs[addr] = time.Now()
	hub.mu.Unlock()
}

func formatSSE(event string, payload interface{}) string {
	b, _ := json.Marshal(payload)
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event, string(b))
}

func (h *Handler) subscribeDevices(schoolID uint) (uint64, <-chan string, func()) {
	hub := h.hub
	if hub == nil {
		ch := make(chan string)
		close(ch)
		return 0, ch, func() {}
	}
	hub.mu.Lock()
	hub.nextID++
	id := hub.nextID
	sub := &liveSubscriber{
		id:       id,
		schoolID: schoolID,
		ch:       make(chan string, 32),
	}
	hub.subs[id] = sub
	startMonitor := !hub.monitorOn
	if startMonitor {
		hub.monitorOn = true
	}
	hub.mu.Unlock()

	if startMonitor {
		go h.runProbeMonitor()
	}

	cancel := func() {
		hub.mu.Lock()
		if s, ok := hub.subs[id]; ok {
			delete(hub.subs, id)
			close(s.ch)
		}
		hub.mu.Unlock()
	}
	return id, sub.ch, cancel
}

func (h *Handler) notifyDeviceHeartbeat(dev *model.Device, isNew bool) {
	if h.hub == nil || dev == nil {
		return
	}
	h.hub.mu.Lock()
	if len(h.hub.subs) == 0 {
		h.hub.mu.Unlock()
		return
	}
	subs := make([]*liveSubscriber, 0, len(h.hub.subs))
	for _, s := range h.hub.subs {
		if s.schoolID == 0 || s.schoolID == dev.SchoolID {
			subs = append(subs, s)
		}
	}
	h.hub.mu.Unlock()
	if len(subs) == 0 {
		return
	}

	msg := formatSSE("heartbeat", map[string]interface{}{
		"id":                     dev.ID,
		"school_id":              dev.SchoolID,
		"is_new":                 isNew,
		"online":                 true,
		"last_seen_at":           dev.LastSeenAt,
		"last_seen_sec":          0,
		"active_conns_count":     dev.ActiveConnsCount,
		"temp_password":          dev.TempPassword,
		"has_permanent_password": dev.HasPermanentPassword,
		"local_ips":              dev.LocalIPs,
		"remote_ip":              dev.RemoteIP,
	})
	for _, s := range subs {
		select {
		case s.ch <- msg:
		default:
		}
	}
}

func (h *Handler) runProbeMonitor() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.hub.mu.Lock()
		if len(h.hub.subs) == 0 {
			h.hub.monitorOn = false
			h.hub.mu.Unlock()
			return
		}
		subs := make([]*liveSubscriber, 0, len(h.hub.subs))
		for _, s := range h.hub.subs {
			subs = append(subs, s)
		}
		h.hub.mu.Unlock()

		var devs []model.Device
		h.DB.Select("id, client_role, school_id, last_seen_at, active_conns_count").Find(&devs)
		ids := make([]string, 0, len(devs))
		for _, d := range devs {
			ids = append(ids, d.ID)
		}
		hbbsStates, hbbsReachable, probeMs := h.probeLiveStates(ids)
		probedAt := time.Now()
		timeoutSec := int(h.offlineThreshold().Seconds())

		newSnap := make(map[string]deviceSnapshot, len(devs))
		for i := range devs {
			h.applyDeviceLiveStatus(&devs[i], hbbsStates, hbbsReachable)
			var hFlag int8 = -1
			if devs[i].HbbsOnline != nil {
				if *devs[i].HbbsOnline {
					hFlag = 1
				} else {
					hFlag = 0
				}
			}
			newSnap[devs[i].ID] = deviceSnapshot{
				online:       devs[i].Online,
				hbbsOnline:   hFlag,
				activeConns:  devs[i].ActiveConnsCount,
				lastSeenUnix: devs[i].LastSeenAt.Unix(),
			}
		}

		h.hub.mu.Lock()
		topologyChanged := len(newSnap) != len(h.hub.lastSnap)
		if !topologyChanged {
			for id := range newSnap {
				if _, ok := h.hub.lastSnap[id]; !ok {
					topologyChanged = true
					break
				}
			}
		}
		h.hub.lastSnap = newSnap
		h.hub.mu.Unlock()

		for _, sub := range subs {
			states := make(map[string]map[string]interface{}, len(devs))
			onlineCount := 0
			totalCount := 0
			for _, d := range devs {
				if sub.schoolID > 0 && d.SchoolID != sub.schoolID {
					continue
				}
				totalCount++
				if d.Online {
					onlineCount++
				}
				entry := map[string]interface{}{
					"online":             d.Online,
					"last_seen_at":       d.LastSeenAt,
					"last_seen_sec":      d.LastSeenSec,
					"active_conns_count": d.ActiveConnsCount,
				}
				if d.HbbsOnline != nil {
					entry["hbbs_online"] = *d.HbbsOnline
				}
				states[d.ID] = entry
			}
			msg := formatSSE("probe", map[string]interface{}{
				"probed_at":             probedAt,
				"hbbs_reachable":        hbbsReachable,
				"probe_ms":              probeMs,
				"heartbeat_timeout_sec": timeoutSec,
				"topology_changed":      topologyChanged,
				"total_count":           totalCount,
				"online_count":          onlineCount,
				"states":                states,
			})
			select {
			case sub.ch <- msg:
			default:
			}
		}
	}
}
