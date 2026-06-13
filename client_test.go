package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient points a Client at an httptest server so the HTTP plumbing,
// envelope decoding, and error handling are exercised without touching Hetzner.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return newClient(Config{BaseURL: ts.URL, Token: "test-token"})
}

func TestServersListDecodesEnvelope(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header = %q, want Bearer test-token", got)
		}
		if r.URL.Path != "/servers" {
			t.Errorf("path = %q, want /servers", r.URL.Path)
		}
		w.Write([]byte(`{"servers":[{"id":42,"name":"web","status":"running","server_type":{"name":"cx22"},"public_net":{"ipv4":{"ip":"1.2.3.4"}}}]}`))
	})

	servers, err := c.servers()
	if err != nil {
		t.Fatalf("servers: %v", err)
	}
	if len(servers) != 1 || servers[0].ID != 42 || servers[0].PublicNet.IPv4.IP != "1.2.3.4" {
		t.Fatalf("unexpected servers: %+v", servers)
	}
}

func TestListAllFollowsPagination(t *testing.T) {
	// Page 1 points at a next page; page 2 ends the walk with next_page null.
	// listAll must concatenate both, so a >1-page account is never truncated.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			w.Write([]byte(`{"servers":[{"id":1,"name":"a"}],"meta":{"pagination":{"page":1,"next_page":2}}}`))
		case "2":
			w.Write([]byte(`{"servers":[{"id":2,"name":"b"}],"meta":{"pagination":{"page":2,"next_page":null}}}`))
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	})

	servers, err := c.servers()
	if err != nil {
		t.Fatalf("servers: %v", err)
	}
	if len(servers) != 2 || servers[0].ID != 1 || servers[1].ID != 2 {
		t.Fatalf("got %+v, want both pages concatenated (ids 1,2)", servers)
	}
}

func TestServerShow(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/servers/7" {
			t.Errorf("path = %q, want /servers/7", r.URL.Path)
		}
		w.Write([]byte(`{"server":{"id":7,"name":"db"}}`))
	})

	s, err := c.server(7)
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	if s.Name != "db" {
		t.Errorf("name = %q, want db", s.Name)
	}
}

func TestLookupIDNumericSkipsHTTP(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("numeric reference must not hit the API")
	})

	id, err := c.lookupID("servers", "99")
	if err != nil || id != 99 {
		t.Fatalf("lookupID(99) = %d, %v; want 99, nil", id, err)
	}
}

func TestLookupIDByName(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "web" {
			t.Errorf("name query = %q, want web", got)
		}
		w.Write([]byte(`{"servers":[{"id":13,"name":"web"}]}`))
	})

	id, err := c.lookupID("servers", "web")
	if err != nil || id != 13 {
		t.Fatalf("lookupID(web) = %d, %v; want 13, nil", id, err)
	}
}

func TestLookupIDByNameIgnoresMeta(t *testing.T) {
	// The real API wraps lists with a "meta" object next to the collection
	// array; the resolver must decode only the array, never the whole map.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"servers":[{"id":21,"name":"web"}],"meta":{"pagination":{"page":1,"per_page":25,"total_entries":1}}}`))
	})

	id, err := c.lookupID("servers", "web")
	if err != nil || id != 21 {
		t.Fatalf("lookupID(web) = %d, %v; want 21, nil (meta object must not break decoding)", id, err)
	}
}

func TestLookupIDByNameNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"servers":[]}`))
	})

	if _, err := c.lookupID("servers", "ghost"); err == nil {
		t.Error("expected an error when no server matches the name")
	}
}

func TestCreateServerReturnsRootPassword(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"server":{"id":100,"name":"fresh"},"root_password":"hunter2"}`))
	})

	server, pw, err := c.createServer(CreateServerRequest{Name: "fresh", ServerType: "cx22", Image: "ubuntu-24.04"})
	if err != nil {
		t.Fatalf("createServer: %v", err)
	}
	if server.ID != 100 || pw != "hunter2" {
		t.Errorf("got id=%d pw=%q, want 100/hunter2", server.ID, pw)
	}
}

func TestDeleteServerAcceptsEmptyBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.deleteServer(5); err != nil {
		t.Errorf("deleteServer: %v", err)
	}
}

func TestErrorEnvelopeSurfacesMessage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"not_found","message":"server not found"}}`))
	})

	_, err := c.server(404)
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
	if got := err.Error(); !strings.Contains(got, "server not found") || !strings.Contains(got, "not_found") {
		t.Errorf("error = %q, want it to include message and code", got)
	}
}

func TestRawPassthroughReturnsStatusAndBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"hello":"world"}`))
	})

	status, body, err := c.raw(http.MethodGet, "/anything", nil)
	if err != nil {
		t.Fatalf("raw: %v", err)
	}
	if status != http.StatusTeapot {
		t.Errorf("status = %d, want 418", status)
	}
	if !strings.Contains(string(body), "world") {
		t.Errorf("body = %q, want it to contain world", string(body))
	}
}
