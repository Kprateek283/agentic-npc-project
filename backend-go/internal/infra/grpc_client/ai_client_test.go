package grpc_client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	pb "agentic-npc-backend/internal/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

// Emotion values are chosen to be exact in float32 (0.5, 0.25, 0.75, ...), so the conversion
// from float64 can be asserted with ==.

type reqArgs struct {
	personality, backstory, lore string
	speaker, mood                map[string]float64
	lines                        []string
	eventType, question, source  string
	step                         int
	rate                         float32
}

func (a reqArgs) build() *pb.EventRequest {
	return buildEventRequest(a.personality, a.backstory, a.lore, a.speaker, a.mood, a.lines,
		a.eventType, a.question, a.source, a.step, a.rate)
}

// wireFields returns the field numbers present in the encoded message, in order, one entry
// per occurrence (a map or repeated field appears once per element).
func wireFields(t *testing.T, b []byte) []protowire.Number {
	t.Helper()
	var nums []protowire.Number
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			t.Fatalf("bad tag: %v", protowire.ParseError(n))
		}
		b = b[n:]
		m := protowire.ConsumeFieldValue(num, typ, b)
		if m < 0 {
			t.Fatalf("bad field %d: %v", num, protowire.ParseError(m))
		}
		b = b[m:]
		nums = append(nums, num)
	}
	return nums
}

func uniqueSorted(nums []protowire.Number) []protowire.Number {
	seen := map[protowire.Number]bool{}
	var out []protowire.Number
	for _, n := range nums {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func countField(nums []protowire.Number, want protowire.Number) int {
	c := 0
	for _, n := range nums {
		if n == want {
			c++
		}
	}
	return c
}

// roundTrip encodes the request as it goes on the wire and decodes it as the Python side would.
func roundTrip(t *testing.T, req *pb.EventRequest) (*pb.EventRequest, []protowire.Number) {
	t.Helper()
	b, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got pb.EventRequest
	if err := proto.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &got, wireFields(t, b)
}

// Field numbers from proto/ai.proto.
const (
	fPersonality protowire.Number = 1
	fBackstory   protowire.Number = 2
	fLore        protowire.Number = 3
	fSpeaker     protowire.Number = 4
	fMood        protowire.Number = 5
	fEventType   protowire.Number = 6
	fQuestion    protowire.Number = 7
	fSource      protowire.Number = 8
	fStep        protowire.Number = 9
	fRate        protowire.Number = 10
	fLines       protowire.Number = 11
)

func TestEventRequestWireFields(t *testing.T) {
	full := reqArgs{
		personality: "gamedata/npcs/elara/personality.json",
		backstory:   "gamedata/npcs/elara/backstory.json",
		lore:        "gamedata/npcs/elara/lore.json",
		speaker:     map[string]float64{"anger": 0.75, "trust": -0.5, "joy": 0.25, "fear": 0.125, "sadness": 0.0625},
		mood:        map[string]float64{"anger": 0.1875, "trust": -0.125},
		lines:       []string{"You triggered PLAYER_ATTACKED on Elara", "Someone gave moonshadow_petal to Elara"},
		eventType:   "PLAYER_ASKED_QUESTION",
		question:    "Do you know of a cure?",
		source:      "player1",
		step:        2,
		rate:        0.5,
	}

	cases := []struct {
		name       string
		args       reqArgs
		wantFields []protowire.Number // distinct field numbers on the wire
		check      func(t *testing.T, got *pb.EventRequest, nums []protowire.Number)
	}{
		{
			name:       "every field reaches its own wire field",
			args:       full,
			wantFields: []protowire.Number{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			check: func(t *testing.T, got *pb.EventRequest, nums []protowire.Number) {
				if got.PersonalityPath != "gamedata/npcs/elara/personality.json" ||
					got.BackstoryPath != "gamedata/npcs/elara/backstory.json" ||
					got.LorePath != "gamedata/npcs/elara/lore.json" {
					t.Errorf("paths = %q, %q, %q", got.PersonalityPath, got.BackstoryPath, got.LorePath)
				}
				if got.EventType != "PLAYER_ASKED_QUESTION" || got.QuestionText != "Do you know of a cure?" || got.SourceEntityId != "player1" {
					t.Errorf("event = %q, %q, %q", got.EventType, got.QuestionText, got.SourceEntityId)
				}
				if got.CurrentQuestStep != 2 || got.CompletionRate != 0.5 {
					t.Errorf("quest step %d, completion rate %v; want 2, 0.5", got.CurrentQuestStep, got.CompletionRate)
				}
				if n := countField(nums, fSpeaker); n != 5 {
					t.Errorf("speaker_emotions entries on the wire = %d, want 5", n)
				}
				if n := countField(nums, fMood); n != 2 {
					t.Errorf("general_mood entries on the wire = %d, want 2", n)
				}
			},
		},
		{
			name:       "the speaker's feelings and the general mood stay in separate fields",
			args:       full,
			wantFields: []protowire.Number{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			check: func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {
				wantSpeaker := map[string]float32{"anger": 0.75, "trust": -0.5, "joy": 0.25, "fear": 0.125, "sadness": 0.0625}
				wantMood := map[string]float32{"anger": 0.1875, "trust": -0.125}
				if !reflect.DeepEqual(got.SpeakerEmotions, wantSpeaker) {
					t.Errorf("speaker_emotions = %v, want %v", got.SpeakerEmotions, wantSpeaker)
				}
				if !reflect.DeepEqual(got.GeneralMood, wantMood) {
					t.Errorf("general_mood = %v, want %v", got.GeneralMood, wantMood)
				}
			},
		},
		{
			name:       "memory lines keep their order, duplicates included",
			args:       reqArgs{lines: []string{"b line", "a line", "b line"}},
			wantFields: []protowire.Number{fLines},
			check: func(t *testing.T, got *pb.EventRequest, nums []protowire.Number) {
				want := []string{"b line", "a line", "b line"}
				if !reflect.DeepEqual(got.MemoryLines, want) {
					t.Errorf("memory_lines = %q, want %q", got.MemoryLines, want)
				}
				if n := countField(nums, fLines); n != 3 {
					t.Errorf("memory_lines entries on the wire = %d, want 3", n)
				}
			},
		},
		{
			name: "an emotion name the protocol never saw travels like any other",
			args: reqArgs{
				speaker: map[string]float64{"awe": 0.5, "trust": 0.25},
				mood:    map[string]float64{"nostalgia": 0.75},
			},
			wantFields: []protowire.Number{fSpeaker, fMood},
			check: func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {
				if got.SpeakerEmotions["awe"] != 0.5 || got.SpeakerEmotions["trust"] != 0.25 || len(got.SpeakerEmotions) != 2 {
					t.Errorf("speaker_emotions = %v, want awe 0.5, trust 0.25", got.SpeakerEmotions)
				}
				if got.GeneralMood["nostalgia"] != 0.75 || len(got.GeneralMood) != 1 {
					t.Errorf("general_mood = %v, want nostalgia 0.75", got.GeneralMood)
				}
			},
		},
		{
			name:       "empty maps and no memory lines put nothing on the wire",
			args:       reqArgs{speaker: map[string]float64{}, mood: map[string]float64{}, lines: []string{}, eventType: "PLAYER_INTERACT"},
			wantFields: []protowire.Number{fEventType},
			check: func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {
				if len(got.SpeakerEmotions) != 0 || len(got.GeneralMood) != 0 || len(got.MemoryLines) != 0 {
					t.Errorf("got %v, %v, %q; want all empty", got.SpeakerEmotions, got.GeneralMood, got.MemoryLines)
				}
			},
		},
		{
			name:       "nil maps are sent as empty, not as an error",
			args:       reqArgs{eventType: "PLAYER_INTERACT"},
			wantFields: []protowire.Number{fEventType},
			check:      func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {},
		},
		{
			name:       "a zero emotion is still sent as an entry, so the other side sees the name",
			args:       reqArgs{speaker: map[string]float64{"anger": 0}},
			wantFields: []protowire.Number{fSpeaker},
			check: func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {
				v, ok := got.SpeakerEmotions["anger"]
				if !ok || v != 0 {
					t.Errorf("speaker_emotions = %v, want anger present at 0", got.SpeakerEmotions)
				}
			},
		},
		{
			name:       "float64 feelings are narrowed to float32 without clamping",
			args:       reqArgs{speaker: map[string]float64{"trust": 0.1, "anger": 1.5}},
			wantFields: []protowire.Number{fSpeaker},
			check: func(t *testing.T, got *pb.EventRequest, _ []protowire.Number) {
				if got.SpeakerEmotions["trust"] != float32(0.1) || got.SpeakerEmotions["anger"] != 1.5 {
					t.Errorf("speaker_emotions = %v, want trust float32(0.1), anger 1.5", got.SpeakerEmotions)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, nums := roundTrip(t, tc.args.build())
			if fields := uniqueSorted(nums); !reflect.DeepEqual(fields, tc.wantFields) {
				t.Errorf("wire fields = %v, want %v", fields, tc.wantFields)
			}
			tc.check(t, got, nums)
		})
	}

	t.Run("the request does not alias the caller's maps", func(t *testing.T) {
		speaker := map[string]float64{"trust": 0.5}
		req := reqArgs{speaker: speaker}.build()
		speaker["trust"] = -1
		speaker["anger"] = 1
		if len(req.SpeakerEmotions) != 1 || req.SpeakerEmotions["trust"] != 0.5 {
			t.Errorf("speaker_emotions = %v after the caller changed its map, want trust 0.5 only", req.SpeakerEmotions)
		}
	})
}

// fakeBrain is an AIBrainClient that records calls and replays scripted results.
type fakeBrain struct {
	thinkErrs   []error // one per call; nil means success
	thinkCalls  int
	lastReq     *pb.EventRequest
	deadlines   []bool
	remaining   []time.Duration
	stream      *fakeStream
	streamErr   error
	streamCalls int
}

func (f *fakeBrain) Think(ctx context.Context, in *pb.EventRequest, _ ...grpc.CallOption) (*pb.ActionResponse, error) {
	f.thinkCalls++
	f.lastReq = in
	dl, ok := ctx.Deadline()
	f.deadlines = append(f.deadlines, ok)
	if ok {
		f.remaining = append(f.remaining, time.Until(dl))
	}
	if i := f.thinkCalls - 1; i < len(f.thinkErrs) && f.thinkErrs[i] != nil {
		return nil, f.thinkErrs[i]
	}
	return &pb.ActionResponse{ActionType: "SPEAK", Content: "Welcome, traveller."}, nil
}

func (f *fakeBrain) ThinkStream(ctx context.Context, in *pb.EventRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.TokenChunk], error) {
	f.streamCalls++
	f.lastReq = in
	_, ok := ctx.Deadline()
	f.deadlines = append(f.deadlines, ok)
	if f.streamErr != nil {
		return nil, f.streamErr
	}
	return f.stream, nil
}

// fakeStream replays chunks, then returns end (io.EOF when nil).
type fakeStream struct {
	grpc.ClientStream
	chunks []*pb.TokenChunk
	end    error
	recvs  int
}

func (s *fakeStream) Recv() (*pb.TokenChunk, error) {
	s.recvs++
	if len(s.chunks) == 0 {
		if s.end != nil {
			return nil, s.end
		}
		return nil, io.EOF
	}
	c := s.chunks[0]
	s.chunks = s.chunks[1:]
	return c, nil
}

func tok(text string) *pb.TokenChunk { return &pb.TokenChunk{Text: text} }
func done(text string) *pb.TokenChunk {
	return &pb.TokenChunk{Text: text, Done: true, ActionType: "SPEAK"}
}
func doneAs(action string) *pb.TokenChunk { return &pb.TokenChunk{Done: true, ActionType: action} }

func callThink(c *AIClient) (*pb.ActionResponse, error) {
	return c.CallAIThink(context.Background(), "p.json", "b.json", "l.json",
		map[string]float64{"trust": 0.5}, map[string]float64{"awe": 0.25}, []string{"You gave apple to Elara"},
		"PLAYER_INTERACT", "", "player1", 1, 0.5)
}

func TestCallAIThink(t *testing.T) {
	unavailable := status.Error(codes.Unavailable, "connection refused")
	cases := []struct {
		name      string
		errs      []error
		wantCalls int
		wantCode  codes.Code // codes.OK means success
	}{
		{"a successful call is made once and returns the response", nil, 1, codes.OK},
		{"one Unavailable is retried and the retry's answer is returned", []error{unavailable}, 2, codes.OK},
		{"a second Unavailable is returned, with no third attempt", []error{unavailable, unavailable}, 2, codes.Unavailable},
		{"a deadline exceeded is never retried", []error{status.Error(codes.DeadlineExceeded, "slow model")}, 1, codes.DeadlineExceeded},
		{"an internal error is not retried", []error{status.Error(codes.Internal, "boom")}, 1, codes.Internal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeBrain{thinkErrs: tc.errs}
			res, err := callThink(&AIClient{client: f, timeout: time.Hour})
			if f.thinkCalls != tc.wantCalls {
				t.Errorf("Think calls = %d, want %d", f.thinkCalls, tc.wantCalls)
			}
			if tc.wantCode == codes.OK {
				if err != nil || res == nil || res.ActionType != "SPEAK" || res.Content != "Welcome, traveller." {
					t.Fatalf("got %v, %v; want the fake's SPEAK response", res, err)
				}
			} else {
				if status.Code(err) != tc.wantCode || res != nil {
					t.Fatalf("got %v, code %v; want nil and %v", res, status.Code(err), tc.wantCode)
				}
			}
			if f.lastReq.SpeakerEmotions["trust"] != 0.5 || f.lastReq.GeneralMood["awe"] != 0.25 ||
				len(f.lastReq.MemoryLines) != 1 || f.lastReq.SourceEntityId != "player1" {
				t.Errorf("request sent = %v, want the built request", f.lastReq)
			}
		})
	}

	t.Run("every attempt carries the configured deadline", func(t *testing.T) {
		f := &fakeBrain{thinkErrs: []error{unavailable}}
		if _, err := callThink(&AIClient{client: f, timeout: time.Hour}); err != nil {
			t.Fatalf("CallAIThink: %v", err)
		}
		if !reflect.DeepEqual(f.deadlines, []bool{true, true}) {
			t.Fatalf("deadline present per attempt = %v, want [true true]", f.deadlines)
		}
		// Both attempts share one deadline of an hour from the call; the retry does not get a
		// fresh one. Only bounds are asserted, since the deadline is set from the wall clock.
		for i, r := range f.remaining {
			if r <= 59*time.Minute || r > time.Hour {
				t.Errorf("attempt %d: time left = %v, want just under 1h", i+1, r)
			}
		}
		if f.remaining[1] > f.remaining[0] {
			t.Errorf("retry had more time (%v) than the first attempt (%v)", f.remaining[1], f.remaining[0])
		}
	})
}

func streamCall(c *AIClient) (string, string, []string, error) {
	var toks []string
	full, action, err := c.CallAIThinkStream(context.Background(), "p.json", "b.json", "l.json",
		map[string]float64{"trust": 0.5}, nil, nil, "PLAYER_ASKED_QUESTION", "What is the Sunstone?", "player1", 0, 0,
		func(s string) { toks = append(toks, s) })
	return full, action, toks, err
}

func TestCallAIThinkStream(t *testing.T) {
	midStream := status.Error(codes.Unavailable, "AI service restarted")
	cases := []struct {
		name       string
		chunks     []*pb.TokenChunk
		end        error
		openErr    error
		wantFull   string
		wantAction string // "" means the default, SPEAK
		wantTokens []string
		wantCode   codes.Code
		wantRecvs  int // -1 to skip
	}{
		{
			name:       "tokens are forwarded in order and joined into the full reply",
			chunks:     []*pb.TokenChunk{tok("The "), tok("Sunstone "), tok("is lost."), done("")},
			wantFull:   "The Sunstone is lost.",
			wantTokens: []string{"The ", "Sunstone ", "is lost."},
			wantRecvs:  4,
		},
		{
			name:       "a whole answer in one chunk arrives as one token",
			chunks:     []*pb.TokenChunk{tok("Leave me be."), done("")},
			wantFull:   "Leave me be.",
			wantTokens: []string{"Leave me be."},
			wantRecvs:  2,
		},
		{
			name:       "empty chunks are not forwarded",
			chunks:     []*pb.TokenChunk{tok(""), tok("Hm."), tok(""), done("")},
			wantFull:   "Hm.",
			wantTokens: []string{"Hm."},
			wantRecvs:  4,
		},
		{
			name:       "the done frame ends the stream and anything after it is not read",
			chunks:     []*pb.TokenChunk{tok("Yes."), done(""), tok("ignored")},
			wantFull:   "Yes.",
			wantTokens: []string{"Yes."},
			wantRecvs:  2,
		},
		{
			name:       "text carried on the done frame itself is kept",
			chunks:     []*pb.TokenChunk{tok("Yes"), done(" and no.")},
			wantFull:   "Yes and no.",
			wantTokens: []string{"Yes", " and no."},
			wantRecvs:  2,
		},
		{
			name:       "the done frame's action type is returned",
			chunks:     []*pb.TokenChunk{tok("Take it."), doneAs("GIVE_ITEM")},
			wantFull:   "Take it.",
			wantAction: "GIVE_ITEM",
			wantTokens: []string{"Take it."},
			wantRecvs:  2,
		},
		{
			name:       "a done frame with no action type falls back to SPEAK",
			chunks:     []*pb.TokenChunk{tok("Hm."), doneAs("")},
			wantFull:   "Hm.",
			wantTokens: []string{"Hm."},
			wantRecvs:  2,
		},
		{
			name:       "a stream that closes without a done frame returns what arrived",
			chunks:     []*pb.TokenChunk{tok("Half an ")},
			wantFull:   "Half an ",
			wantTokens: []string{"Half an "},
			wantRecvs:  2,
		},
		{
			name:       "a mid-stream failure returns the partial text and the error",
			chunks:     []*pb.TokenChunk{tok("I was about to ")},
			end:        midStream,
			wantFull:   "I was about to ",
			wantTokens: []string{"I was about to "},
			wantCode:   codes.Unavailable,
			wantRecvs:  2,
		},
		{
			name:      "a stream that cannot open returns no text and is not retried",
			openErr:   status.Error(codes.Unavailable, "connection refused"),
			wantCode:  codes.Unavailable,
			wantRecvs: -1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeStream{chunks: tc.chunks, end: tc.end}
			f := &fakeBrain{stream: s, streamErr: tc.openErr}
			full, action, toks, err := streamCall(&AIClient{client: f, timeout: time.Hour})
			if full != tc.wantFull {
				t.Errorf("full = %q, want %q", full, tc.wantFull)
			}
			wantAction := tc.wantAction
			if wantAction == "" {
				wantAction = "SPEAK"
			}
			if action != wantAction {
				t.Errorf("action = %q, want %q", action, wantAction)
			}
			if len(toks) != 0 || len(tc.wantTokens) != 0 {
				if !reflect.DeepEqual(toks, tc.wantTokens) {
					t.Errorf("tokens = %q, want %q", toks, tc.wantTokens)
				}
			}
			if tc.wantCode == codes.OK && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			if tc.wantCode != codes.OK && status.Code(err) != tc.wantCode {
				t.Errorf("err = %v, want code %v", err, tc.wantCode)
			}
			if f.streamCalls != 1 {
				t.Errorf("ThinkStream calls = %d, want 1", f.streamCalls)
			}
			if tc.wantRecvs >= 0 && s.recvs != tc.wantRecvs {
				t.Errorf("Recv calls = %d, want %d", s.recvs, tc.wantRecvs)
			}
			if !reflect.DeepEqual(f.deadlines, []bool{true}) {
				t.Errorf("deadline present = %v, want [true]", f.deadlines)
			}
			if f.lastReq.QuestionText != "What is the Sunstone?" || f.lastReq.SpeakerEmotions["trust"] != 0.5 {
				t.Errorf("request sent = %v, want the built request", f.lastReq)
			}
		})
	}

	t.Run("a non-status error from Recv is passed through unchanged", func(t *testing.T) {
		boom := errors.New("boom")
		f := &fakeBrain{stream: &fakeStream{end: boom}}
		full, _, toks, err := streamCall(&AIClient{client: f, timeout: time.Hour})
		if full != "" || len(toks) != 0 || !errors.Is(err, boom) {
			t.Errorf("got %q, %q, %v; want empty and boom", full, toks, err)
		}
	})
}

// The two services each keep a copy of the contract. If they drift, one side encodes fields the
// other decodes as something else, and nothing fails until a reply sounds wrong.
func TestProtoContractCopiesMatch(t *testing.T) {
	goSide, err := os.ReadFile("../../../proto/ai.proto")
	if err != nil {
		t.Fatalf("read Go copy: %v", err)
	}
	pySide, err := os.ReadFile("../../../../ai-service-python/proto/ai.proto")
	if err != nil {
		t.Fatalf("read Python copy: %v", err)
	}
	if !bytes.Equal(goSide, pySide) {
		t.Fatal("backend-go/proto/ai.proto and ai-service-python/proto/ai.proto differ")
	}
}
