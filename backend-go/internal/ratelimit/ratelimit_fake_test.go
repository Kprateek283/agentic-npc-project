package ratelimit

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// fakeRedis is a tiny in-process Redis that understands just what Allow sends: MULTI, INCR,
// EXPIRE [NX] and EXEC. The real go-redis client talks to it through net.Pipe, so the wire
// protocol is exercised but nothing listens on a port. Its clock only moves when a test calls
// advance, so windows can be asserted to the second.
type fakeRedis struct {
	mu      sync.Mutex
	now     time.Time
	vals    map[string]int64
	expires map[string]time.Time
	cmds    [][]string // every command received, lower-cased name first
	incrErr string     // when set, INCR replies with this error
	dialErr error      // when set, connecting fails
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		now:     time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		vals:    map[string]int64{},
		expires: map[string]time.Time{},
	}
}

func (f *fakeRedis) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

func (f *fakeRedis) client(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:       "fake:6379",
		MaxRetries: -1,
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if f.dialErr != nil {
				return nil, f.dialErr
			}
			client, server := net.Pipe()
			go f.serve(server)
			return client, nil
		},
	})
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func (f *fakeRedis) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	var queued [][]string
	inTx := false
	for {
		args, err := readCommand(r)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.cmds = append(f.cmds, args)
		f.mu.Unlock()

		var reply string
		switch {
		case args[0] == "multi":
			inTx, queued = true, nil
			reply = "+OK\r\n"
		case args[0] == "exec":
			var b strings.Builder
			fmt.Fprintf(&b, "*%d\r\n", len(queued))
			for _, q := range queued {
				b.WriteString(f.exec(q))
			}
			inTx, queued = false, nil
			reply = b.String()
		case inTx:
			queued = append(queued, args)
			reply = "+QUEUED\r\n"
		default:
			reply = f.exec(args)
		}
		if _, err := io.WriteString(conn, reply); err != nil {
			return
		}
	}
}

func (f *fakeRedis) exec(args []string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := ""
	if len(args) > 1 {
		key = args[1]
		if exp, ok := f.expires[key]; ok && !f.now.Before(exp) {
			delete(f.vals, key)
			delete(f.expires, key)
		}
	}
	switch args[0] {
	case "incr":
		if f.incrErr != "" {
			return "-" + f.incrErr + "\r\n"
		}
		f.vals[key]++
		return fmt.Sprintf(":%d\r\n", f.vals[key])
	case "expire":
		secs, _ := strconv.Atoi(args[2])
		if _, ok := f.vals[key]; !ok {
			return ":0\r\n"
		}
		if len(args) > 3 && strings.EqualFold(args[3], "nx") {
			if _, has := f.expires[key]; has {
				return ":0\r\n"
			}
		}
		f.expires[key] = f.now.Add(time.Duration(secs) * time.Second)
		return ":1\r\n"
	case "ping":
		return "+PONG\r\n"
	}
	return "-ERR unknown command '" + args[0] + "'\r\n"
}

// readCommand reads one RESP array of bulk strings.
func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("expected array, got %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, err
	}
	args := make([]string, n)
	for i := range args {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(strings.TrimSpace(hdr[1:]))
		if err != nil {
			return nil, err
		}
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:size])
	}
	args[0] = strings.ToLower(args[0])
	return args, nil
}

func (f *fakeRedis) commands() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.cmds...)
}

// expiresIn is how long until key expires on the fake's clock, or -1 if it has no expiry.
func (f *fakeRedis) expiresIn(key string) time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	exp, ok := f.expires[key]
	if !ok {
		return -1
	}
	return exp.Sub(f.now)
}

func TestAllowWindowOffline(t *testing.T) {
	type call struct {
		after       time.Duration // advance the fake clock by this much first
		player      string
		wantAllowed bool
		wantCount   int64
	}
	cases := []struct {
		name  string
		limit int
		calls []call
	}{
		{
			name:  "calls up to the limit are allowed and the next one is refused",
			limit: 3,
			calls: []call{{0, "p1", true, 1}, {0, "p1", true, 2}, {0, "p1", true, 3}, {0, "p1", false, 4}},
		},
		{
			name:  "refused calls keep counting inside the window",
			limit: 3,
			calls: []call{{0, "p1", true, 1}, {0, "p1", true, 2}, {0, "p1", true, 3}, {0, "p1", false, 4}, {0, "p1", false, 5}},
		},
		{
			name:  "a limit of one allows exactly one call",
			limit: 1,
			calls: []call{{0, "p1", true, 1}, {0, "p1", false, 2}},
		},
		{
			name:  "one second before the window ends the player is still limited",
			limit: 3,
			calls: []call{
				{0, "p1", true, 1}, {0, "p1", true, 2}, {0, "p1", true, 3},
				{59 * time.Second, "p1", false, 4},
			},
		},
		{
			name:  "when the window ends the count starts again at one",
			limit: 3,
			calls: []call{
				{0, "p1", true, 1}, {0, "p1", true, 2}, {0, "p1", true, 3}, {0, "p1", false, 4},
				{60 * time.Second, "p1", true, 1},
			},
		},
		{
			name:  "the window is fixed from the first call and later calls do not extend it",
			limit: 3,
			calls: []call{
				{0, "p1", true, 1},
				{30 * time.Second, "p1", true, 2},
				{29 * time.Second, "p1", true, 3}, // 59s after the first call
				{1 * time.Second, "p1", true, 1},  // 60s: a new window, not 60s after the last call
			},
		},
		{
			name:  "each player has their own window",
			limit: 1,
			calls: []call{{0, "p1", true, 1}, {0, "p2", true, 1}, {0, "p1", false, 2}, {0, "p2", false, 2}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeRedis()
			rdb := f.client(t)
			for i, c := range tc.calls {
				f.advance(c.after)
				allowed, count, err := Allow(context.Background(), rdb, "ratelimit:llm:"+c.player, tc.limit, time.Minute)
				if err != nil {
					t.Fatalf("call %d: %v", i+1, err)
				}
				if allowed != c.wantAllowed || count != c.wantCount {
					t.Fatalf("call %d (%s): allowed=%v count=%d, want allowed=%v count=%d",
						i+1, c.player, allowed, count, c.wantAllowed, c.wantCount)
				}
			}
		})
	}
}

func TestAllowWireCommands(t *testing.T) {
	cases := []struct {
		name     string
		window   time.Duration
		wantSecs string
	}{
		{"a one-minute window is sent as 60 seconds in a transaction", time.Minute, "60"},
		{"a sub-second window is rounded up to one second by the client", 500 * time.Millisecond, "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeRedis()
			if _, _, err := Allow(context.Background(), f.client(t), "ratelimit:llm:p1", 3, tc.window); err != nil {
				t.Fatalf("Allow: %v", err)
			}
			want := [][]string{
				{"multi"},
				{"incr", "ratelimit:llm:p1"},
				{"expire", "ratelimit:llm:p1", tc.wantSecs, "NX"},
				{"exec"},
			}
			if got := f.commands(); !reflect.DeepEqual(got, want) {
				t.Errorf("commands = %q, want %q", got, want)
			}
		})
	}

	t.Run("the expiry set by the first call is not moved by later calls", func(t *testing.T) {
		f := newFakeRedis()
		rdb := f.client(t)
		ctx := context.Background()
		if _, _, err := Allow(ctx, rdb, "k", 3, time.Minute); err != nil {
			t.Fatalf("Allow: %v", err)
		}
		f.advance(40 * time.Second)
		if _, _, err := Allow(ctx, rdb, "k", 3, time.Minute); err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if got := f.expiresIn("k"); got != 20*time.Second {
			t.Errorf("key expires in %v, want 20s", got)
		}
	})
}

func TestAllowFailureSignals(t *testing.T) {
	cases := []struct {
		name    string
		limit   int
		setup   func(f *fakeRedis)
		nilRDB  bool
		wantOK  bool
		wantErr bool
		wantCmd bool // whether any command should reach Redis
	}{
		{name: "a limit of zero allows without touching Redis", limit: 0, wantOK: true, wantCmd: false},
		{name: "a negative limit allows without touching Redis", limit: -1, wantOK: true, wantCmd: false},
		{name: "a nil client with a limit is an error, not a silent allow", limit: 3, nilRDB: true, wantErr: true},
		{
			name:    "an unreachable Redis returns an error so the caller can fail open",
			limit:   3,
			setup:   func(f *fakeRedis) { f.dialErr = errors.New("connection refused") },
			wantErr: true,
		},
		{
			name:    "an error reply from INCR is returned, not counted",
			limit:   3,
			setup:   func(f *fakeRedis) { f.incrErr = "WRONGTYPE Operation against a key holding the wrong kind of value" },
			wantErr: true,
			wantCmd: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeRedis()
			if tc.setup != nil {
				tc.setup(f)
			}
			var rdb *redis.Client
			if !tc.nilRDB {
				rdb = f.client(t)
			}
			allowed, count, err := Allow(context.Background(), rdb, "ratelimit:llm:p1", tc.limit, time.Minute)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tc.wantErr)
			}
			if allowed != tc.wantOK || count != 0 {
				t.Errorf("allowed=%v count=%d, want allowed=%v count=0", allowed, count, tc.wantOK)
			}
			if got := len(f.commands()) > 0; got != tc.wantCmd {
				t.Errorf("commands reached Redis = %v (%q), want %v", got, f.commands(), tc.wantCmd)
			}
		})
	}
}
