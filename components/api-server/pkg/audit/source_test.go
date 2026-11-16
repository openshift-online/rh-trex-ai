package audit

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/config"
)

func serviceTrusting(t *testing.T, proxies ...string) *Service {
	t.Helper()
	cfg := config.NewAuditConfig()
	cfg.TrustedProxies = proxies
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	return New(cfg, NewMemoryRecorder())
}

func TestResolveSource(t *testing.T) {
	cases := []struct {
		name    string
		trusted []string
		peer    string
		xff     []string
		want    string
	}{
		{"no proxies trusted ignores the header", nil, "10.128.0.5:4000", []string{"203.0.113.9"}, "10.128.0.5"},
		{"untrusted peer cannot spoof", []string{"10.128.0.0/14"}, "198.51.100.7:4000", []string{"203.0.113.9"}, "198.51.100.7"},
		{"one trusted proxy", []string{"10.128.0.0/14"}, "10.128.0.5:4000", []string{"203.0.113.9"}, "203.0.113.9"},
		{"client-supplied prefix is skipped", []string{"10.128.0.0/14"}, "10.128.0.5:4000", []string{"1.2.3.4, 203.0.113.9"}, "203.0.113.9"},
		{"two trusted hops", []string{"10.128.0.0/14"}, "10.128.0.5:4000", []string{"203.0.113.9, 10.129.3.3"}, "203.0.113.9"},
		{"garbage hop stops the walk", []string{"10.128.0.0/14"}, "10.128.0.5:4000", []string{"203.0.113.9, not-an-ip"}, "10.128.0.5"},
		{"all hops trusted uses the leftmost", []string{"10.0.0.0/8"}, "10.0.0.1:4000", []string{"10.0.0.9, 10.0.0.8"}, "10.0.0.9"},
		{"bare IP entry", []string{"10.128.0.5"}, "10.128.0.5:4000", []string{"203.0.113.9"}, "203.0.113.9"},
		{"multiple header lines", []string{"10.128.0.0/14"}, "10.128.0.5:4000", []string{"1.2.3.4", "203.0.113.9"}, "203.0.113.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := serviceTrusting(t, tc.trusted...).resolveSource(tc.peer, tc.xff, "ua")
			if src.IP != tc.want {
				t.Fatalf("ip %q, want %q", src.IP, tc.want)
			}
			if src.PeerIP != strings.Split(tc.peer, ":")[0] {
				t.Fatalf("peer_ip %q", src.PeerIP)
			}
		})
	}
}

func TestForwardedForIsRecordedAndBounded(t *testing.T) {
	s := serviceTrusting(t)
	src := s.resolveSource("10.0.0.1:1", []string{"1.1.1.1, 2.2.2.2", "3.3.3.3"}, "")
	if !reflect.DeepEqual(src.ForwardedFor, []string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("got %v", src.ForwardedFor)
	}
	long := strings.TrimSuffix(strings.Repeat("9.9.9.9,", 50), ",")
	if got := s.resolveSource("10.0.0.1:1", []string{long}, "").ForwardedFor; len(got) != maxForwardedHops {
		t.Fatalf("want %d hops, got %d", maxForwardedHops, len(got))
	}
}

func TestHTTPEventUsesTrustedProxies(t *testing.T) {
	s := serviceTrusting(t, "10.128.0.0/14")
	rec := s.Recorder().(*MemoryRecorder)
	router := newRouter(s, fakeAuthorizer{allowAll}, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	req := httptest.NewRequest(http.MethodPost, "/api/x/v1/dinosaurs", nil)
	req.RemoteAddr = "10.128.0.5:4000"
	req.Header.Set("X-User", "alice")
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.9")
	router.ServeHTTP(httptest.NewRecorder(), req)
	e := rec.Events()[0]
	if e.Source.IP != "203.0.113.9" || e.Source.PeerIP != "10.128.0.5" || len(e.Source.ForwardedFor) != 2 {
		t.Fatalf("unexpected source: %+v", e.Source)
	}
	b, _ := MarshalLine(e)
	if !strings.Contains(string(b), `"source":{"ip":"203.0.113.9","peer_ip":"10.128.0.5","forwarded_for":["1.2.3.4","203.0.113.9"]`) {
		t.Fatalf("unexpected line: %s", b)
	}
	if b, _ := MarshalLine(Event{}); !strings.Contains(string(b), `"forwarded_for":[]`) {
		t.Fatalf("forwarded_for must encode as [] when empty: %s", b)
	}
}

func TestUndeclaredReadKinds(t *testing.T) {
	RecordReads("test_typo_kind", "test_declared_kind")
	got := UndeclaredReadKinds([]string{"test_declared_kind", "dinosaurs"})
	found := false
	for _, k := range got {
		if k == "test_declared_kind" {
			t.Fatalf("declared kind reported: %v", got)
		}
		if k == "test_typo_kind" {
			found = true
		}
	}
	if !found {
		t.Fatalf("typo kind not reported: %v", got)
	}
}
