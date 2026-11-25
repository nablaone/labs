package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"szept/proto"
)

type env struct {
	t   *testing.T
	srv *Server
	ts  *httptest.Server
}

func newEnv(t *testing.T) *env {
	s, err := New(Config{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s)
	t.Cleanup(func() { ts.Close(); s.Close() })
	s.cfg.PublicURL = ts.URL
	return &env{t: t, srv: s, ts: ts}
}

func (e *env) do(method, path, auth string, body any) (*http.Response, []byte) {
	e.t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		r = bytes.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		r = bytes.NewReader(j)
	}
	url := path
	if !strings.HasPrefix(path, "http") {
		url = e.ts.URL + path
	}
	req, _ := http.NewRequest(method, url, r)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func (e *env) expect(code int, method, path, auth string, body any) []byte {
	e.t.Helper()
	resp, out := e.do(method, path, auth, body)
	if resp.StatusCode != code {
		e.t.Fatalf("%s %s: got %d want %d: %s", method, path, resp.StatusCode, code, out)
	}
	return out
}

type device struct {
	token    []byte
	killCode string
}

func (d device) bearer() string { return "Bearer " + b64.EncodeToString(d.token) }

func sendAuth(t []byte) string { return "Send " + b64.EncodeToString(t) }

func (e *env) register(invite []byte) device {
	e.t.Helper()
	d := device{token: randBytes(32), killCode: proto.NewKillCode()}
	kh, _ := proto.KillHash(d.killCode)
	th := sha256.Sum256(d.token)
	e.expect(201, "POST", "/v1/devices", "", map[string]any{
		"invite": b64bytes(invite), "token_hash": b64bytes(th[:]), "kill_hash": b64bytes(kh),
	})
	return d
}

func (e *env) adminInvite() []byte {
	inv, _, err := e.srv.store.CreateInvite(context.Background(), 0, 0, time.Hour)
	if err != nil {
		e.t.Fatal(err)
	}
	return inv
}

func (e *env) mailbox(d device, id, sendToken []byte) {
	e.t.Helper()
	h := sha256.Sum256(sendToken)
	e.expect(201, "PUT", "/v1/mailboxes/"+hex.EncodeToString(id), d.bearer(), map[string]any{"send_hash": b64bytes(h[:])})
}

func (e *env) post(id, sendToken, envelope []byte) {
	e.t.Helper()
	e.expect(202, "POST", "/v1/mailboxes/"+hex.EncodeToString(id)+"/messages", sendAuth(sendToken), envelope)
}

func (e *env) inbox(d device) []wireMsg {
	e.t.Helper()
	var r struct{ Msgs []wireMsg }
	json.Unmarshal(e.expect(200, "GET", "/v1/inbox", d.bearer(), nil), &r)
	return r.Msgs
}

// TestEndToEnd runs two phones through invite, pairing, messaging, files
// and kill against the real HTTP server.
func TestEndToEnd(t *testing.T) {
	e := newEnv(t)

	// Alice joins with an admin invite, then invites Bob.
	inv := e.adminInvite()
	alice := e.register(inv)
	e.expect(403, "POST", "/v1/devices", "", map[string]any{ // invites are single use
		"invite": b64bytes(inv), "token_hash": b64bytes(randBytes(32)),
	})
	var ir struct{ Invite b64bytes }
	json.Unmarshal(e.expect(201, "POST", "/v1/invites", alice.bearer(), nil), &ir)
	bob := e.register(ir.Invite)

	// In-person pairing.
	pa, _ := proto.NewPairing(e.ts.URL, "Alice", nil)
	pb, _ := proto.NewPairing(e.ts.URL, "Bob", nil)
	ka, _ := pa.Complete(&pb.Offer)
	kb, _ := pb.Complete(&pa.Offer)
	e.mailbox(alice, ka.RecvMbox, ka.RecvToken)
	e.mailbox(bob, kb.RecvMbox, kb.RecvToken)
	e.expect(409, "PUT", "/v1/mailboxes/"+hex.EncodeToString(ka.RecvMbox), bob.bearer(), map[string]any{"send_hash": b64bytes(randBytes(32))})

	aSend := &proto.SendChain{CK: ka.SendCK}
	bRecv := &proto.RecvChain{CK: kb.RecvCK}

	// Polling path.
	env1, _ := proto.SealPairwise(aSend, ka.SendMbox, []byte("hello bob"))
	e.post(ka.SendMbox, ka.SendToken, env1)
	msgs := e.inbox(bob)
	if len(msgs) != 1 || len(e.inbox(alice)) != 0 {
		t.Fatalf("inbox: %d", len(msgs))
	}
	m, err := proto.OpenPairwise(bRecv, kb.RecvMbox, msgs[0].Body)
	if err != nil || string(m) != "hello bob" {
		t.Fatal("decrypt", err)
	}
	e.expect(204, "POST", "/v1/inbox/ack", bob.bearer(), ackReq{IDs: []b64bytes{msgs[0].ID}})
	if len(e.inbox(bob)) != 0 {
		t.Fatal("ack did not delete")
	}

	// Errors.
	mb := "/v1/mailboxes/" + hex.EncodeToString(ka.SendMbox) + "/messages"
	e.expect(403, "POST", mb, sendAuth(randBytes(32)), env1)
	e.expect(404, "POST", "/v1/mailboxes/"+hex.EncodeToString(randBytes(16))+"/messages", sendAuth(ka.SendToken), env1)
	e.expect(413, "POST", mb, sendAuth(ka.SendToken), make([]byte, proto.MaxEnvelope+1))
	e.expect(401, "GET", "/v1/inbox", "Bearer "+b64.EncodeToString(randBytes(32)), nil)

	// WebSocket path: queued message is delivered on connect, live one after.
	env2, _ := proto.SealPairwise(aSend, ka.SendMbox, []byte("queued"))
	e.post(ka.SendMbox, ka.SendToken, env2)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.ts.URL, "http")+"/v1/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {bob.bearer()}}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	readMsg := func() wireMsg {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var w wireMsg
		json.Unmarshal(data, &w)
		return w
	}
	w1 := readMsg()
	env3, _ := proto.SealPairwise(aSend, ka.SendMbox, []byte("live"))
	e.post(ka.SendMbox, ka.SendToken, env3)
	w2 := readMsg()
	for i, w := range []wireMsg{w1, w2} {
		m, err := proto.OpenPairwise(bRecv, kb.RecvMbox, w.Body)
		if err != nil || string(m) != []string{"queued", "live"}[i] {
			t.Fatal("ws decrypt", i, err, string(m))
		}
	}
	ack, _ := json.Marshal(ackReq{Type: "ack", IDs: []b64bytes{w1.ID, w2.ID}})
	c.Write(ctx, websocket.MessageText, ack)
	deadline := time.Now().Add(2 * time.Second)
	for len(e.inbox(bob)) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("ws ack did not delete")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Files: Alice uploads, Bob downloads once; then it's gone.
	photo := bytes.Repeat([]byte("jpeg"), 50000)
	fkey := proto.NewFileKey()
	var ct bytes.Buffer
	sum, _ := proto.EncryptFile(fkey, bytes.NewReader(photo), &ct)
	var up struct{ URL string }
	json.Unmarshal(e.expect(201, "POST", "/v1/blobs", alice.bearer(), ct.Bytes()), &up)
	got := e.expect(200, "GET", up.URL, "", nil)
	if s := sha256.Sum256(got); !bytes.Equal(s[:], sum) {
		t.Fatal("blob hash")
	}
	var pt bytes.Buffer
	if err := proto.DecryptFile(fkey, bytes.NewReader(got), &pt); err != nil || !bytes.Equal(pt.Bytes(), photo) {
		t.Fatal("blob decrypt", err)
	}
	e.expect(404, "GET", up.URL, "", nil)

	// Kill code: wrong code does nothing, right code removes Alice.
	e.expect(204, "POST", "/v1/kill", "", map[string]string{"code": "abandon abandon abandon abandon abandon abandon"})
	e.expect(200, "GET", "/v1/inbox", alice.bearer(), nil)
	e.expect(204, "POST", "/v1/kill", "", map[string]string{"code": strings.ToUpper(alice.killCode)})
	e.expect(401, "GET", "/v1/inbox", alice.bearer(), nil)
	// Bob now learns Alice is gone: her mailbox no longer exists.
	env4, _ := proto.SealPairwise(&proto.SendChain{CK: kb.SendCK}, kb.SendMbox, []byte("you there?"))
	e.expect(404, "POST", "/v1/mailboxes/"+hex.EncodeToString(kb.SendMbox)+"/messages", sendAuth(kb.SendToken), env4)
}

func TestExpiry(t *testing.T) {
	e := newEnv(t)
	d := e.register(e.adminInvite())
	id, tok := randBytes(16), randBytes(32)
	e.mailbox(d, id, tok)
	e.post(id, tok, []byte{1, 2, 3})
	if _, err := e.srv.store.Expire(context.Background(), -time.Second); err != nil {
		t.Fatal(err)
	}
	if len(e.inbox(d)) != 0 {
		t.Fatal("message not expired")
	}
}

func TestPushURLValidation(t *testing.T) {
	s := &Server{cfg: Config{PushAllow: []string{"https://ntfy.home.lan/"}}}
	for u, ok := range map[string]bool{
		"":                             true,
		"https://ntfy.home.lan/up123":  true,
		"https://evil.example/x":       false,
		"http://ntfy.home.lan/up123":   false,
		"https://ntfy.home.lan.evil/x": false,
	} {
		if s.validPushURL(u) != ok {
			t.Errorf("%q: want %v", u, ok)
		}
	}
}
