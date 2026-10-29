package main

const apiDiscoveryTest = `package oidc

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestAPIDiscoveryRejectsInvalidMetadata(t *testing.T) {
    for _, tc := range []struct { name, body string; status int }{
        {"missing issuer", "{}", 200},
        {"malformed JSON", "not JSON", 200},
        {"insecure issuer", "{\"issuer_url\":\"http://login.example.com\"}", 200},
        {"issuer userinfo", "{\"issuer_url\":\"https://user:secret@login.example.com\"}", 200},
        {"issuer query", "{\"issuer_url\":\"https://login.example.com?secret=value\"}", 200},
        {"oversized", strings.Repeat(" ", maxResponseBytes+1), 200},
        {"not supported", "", 404},
        {"redirect", "", 302},
        {"server error", "", 500},
    } {
        t.Run(tc.name, func(t *testing.T) {
            api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if r.Header.Get("Authorization") != "" { t.Error("unexpected credentials") }
                w.Header().Set("Location", "/redirect-target")
                w.WriteHeader(tc.status)
                _, _ = w.Write([]byte(tc.body))
            }))
            defer api.Close()
            if _, _, err := DiscoverAPI(context.Background(), api.URL, "/api/v1", false); err == nil { t.Fatal("expected error") }
        })
    }
}

func TestAPIDiscoveryTLSAndCancellation(t *testing.T) {
    api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/proxy/api/v1/auth/config" { t.Errorf("path = %s", r.URL.Path) }
        _, _ = w.Write([]byte("{\"issuer_url\":\"https://login.example.com/realms/test\"}"))
    }))
    defer api.Close()
    if _, _, err := DiscoverAPI(context.Background(), api.URL+"/proxy/api/v1", "/api/v1", false); err == nil { t.Fatal("untrusted TLS accepted") }
    issuer, base, err := DiscoverAPI(context.Background(), api.URL+"/proxy/api/v1/", "/api/v1", true)
    if err != nil || issuer != "https://login.example.com/realms/test" || base != api.URL+"/proxy" { t.Fatalf("issuer=%s base=%s err=%v", issuer, base, err) }
    ctx, cancel := context.WithCancel(context.Background())
    cancel()
    if _, _, err := DiscoverAPI(ctx, api.URL, "/api/v1", true); err == nil { t.Fatal("canceled request accepted") }
}

func TestAPIDiscoveryRejectsUnsafeURLs(t *testing.T) {
    for _, raw := range []string{"", "/relative", "http://api.example.com", "https://user:secret@api.example.com", "https://api.example.com?token=secret", "https://api.example.com#fragment", "ftp://api.example.com"} {
        if _, _, err := DiscoverAPI(context.Background(), raw, "/api/v1", false); err == nil { t.Errorf("accepted %q", raw) }
    }
}
`
