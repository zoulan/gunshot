package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newEngine(t *testing.T, r Runner) *Engine {
	t.Helper()
	e, err := Open(t.TempDir(), r)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func importTest(t *testing.T, e *Engine, quality string) *Job {
	t.Helper()
	r := Request{Account: "a@example.com", Quality: quality, Resources: []Resource{{Name: "photo.jpg", Size: 3}}}
	v, err := e.begin(r, "photos")
	if err != nil {
		t.Fatal(err)
	}
	j := e.find(v.(map[string]any)["id"].(string))
	if err = e.appendChunk(j, Request{Index: 0, Offset: 0, Data: []byte("abc")}); err != nil {
		t.Fatal(err)
	}
	if _, err = e.seal(j); err != nil {
		t.Fatal(err)
	}
	return j
}
func waitIdle(t *testing.T, e *Engine) {
	t.Helper()
	done := make(chan struct{})
	go func() { e.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker stuck")
	}
}
func TestImportBoundsAndDuplicate(t *testing.T) {
	e := newEngine(t, nil)
	for _, name := range []string{"../x", "a/b", "..", "a\\b", "a\x00b"} {
		if _, err := e.begin(Request{Account: "a", Quality: "original", Resources: []Resource{{name, 3}}}, "photos"); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	a := importTest(t, e, "original")
	b := importTest(t, e, "original")
	c := importTest(t, e, "quota")
	if a.State != "pending" || b.State != "cancelled" || c.State != "pending" {
		t.Fatalf("wrong dedup states: %s/%s/%s", a.State, b.State, c.State)
	}
	st, _ := os.Stat(filepath.Join(e.root, "state.json"))
	if st.Mode().Perm() != 0600 {
		t.Fatal("state not private")
	}
}
func TestPartialImportNeverQueues(t *testing.T) {
	e := newEngine(t, nil)
	v, err := e.begin(Request{Account: "a", Quality: "original", Resources: []Resource{{"a.jpg", 3}}}, "photos")
	if err != nil {
		t.Fatal(err)
	}
	j := e.find(v.(map[string]any)["id"].(string))
	if err = e.appendChunk(j, Request{Offset: 1, Data: []byte("a")}); err == nil {
		t.Fatal("accepted invalid offset")
	}
	if _, err = e.seal(j); err == nil {
		t.Fatal("accepted incomplete file")
	}
	reopened, err := Open(e.root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.find(j.ID).State != "cancelled" {
		t.Fatal("interrupted import not cancelled")
	}
	if _, err = os.Stat(e.jobDir(j.ID)); !os.IsNotExist(err) {
		t.Fatal("interrupted staging not removed")
	}
}
func TestRestartCommitIsUncertain(t *testing.T) {
	e := newEngine(t, nil)
	j := importTest(t, e, "original")
	j.State = "committing"
	if err := e.save(); err != nil {
		t.Fatal(err)
	}
	next, err := Open(e.root, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := next.find(j.ID)
	if got.State != "failed" || got.Error != "commit_outcome_unknown" {
		t.Fatalf("unsafe recovery: %+v", got)
	}
}
func TestRestrictionsRetryAndSanitizedError(t *testing.T) {
	called := 0
	e := newEngine(t, func(context.Context, []string, string, string, func(Progress)) (string, error) {
		called++
		return "", errors.New("secret=TOKEN_DONT_LEAK")
	})
	j := importTest(t, e, "original")
	e.Tick()
	if called != 0 {
		t.Fatal("started while offline")
	}
	e.online = true
	e.Tick()
	if called != 0 {
		t.Fatal("started without wifi")
	}
	e.wifi = true
	e.Tick()
	waitIdle(t, e)
	if j.State != "pending" || j.Next <= time.Now().Unix() {
		t.Fatal("missing backoff")
	}
	b, _ := os.ReadFile(filepath.Join(e.root, "state.json"))
	if strings.Contains(string(b), "TOKEN_DONT_LEAK") {
		t.Fatal("leaked error")
	}
	e.state.Options.Retries = 0
	j.Next = 0
	e.Tick()
	waitIdle(t, e)
	if j.State != "failed" {
		t.Fatal("retry limit ignored")
	}
}
func TestCancelAndCommitRace(t *testing.T) {
	started := make(chan struct{})
	e := newEngine(t, func(ctx context.Context, _ []string, _ string, _ string, cb func(Progress)) (string, error) {
		cb(Progress{State: "committing"})
		close(started)
		<-ctx.Done()
		return "remote-key", nil
	})
	j := importTest(t, e, "original")
	e.online = true
	e.wifi = true
	e.Tick()
	<-started
	e.mu.Lock()
	_, err := e.handle(Request{Op: "cancel", ID: j.ID}, "photos")
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	waitIdle(t, e)
	if j.State != "completed" {
		t.Fatal("successful commit incorrectly labelled cancelled")
	}
}
func TestUncertainCommitDoesNotAutoRetry(t *testing.T) {
	e := newEngine(t, func(ctx context.Context, _ []string, _ string, _ string, cb func(Progress)) (string, error) {
		cb(Progress{State: "committing"})
		return "", errors.New("disconnect")
	})
	j := importTest(t, e, "original")
	e.online = true
	e.wifi = true
	e.Tick()
	waitIdle(t, e)
	if j.State != "failed" || j.Error != "commit_outcome_unknown" {
		t.Fatal("uncertain commit scheduled for automatic retry")
	}
}
func TestRoleCannotBeSpoofedInJSON(t *testing.T) {
	e := newEngine(t, nil)
	for _, r := range []struct{ role, body string }{{"unknown", `{"op":"ping","role":"settings"}`}, {"photos", `{"op":"account_add","secret":"foo"}`}, {"photos", `{"op":"configure"}`}, {"settings", `{"op":"conditions","online":true}`}} {
		var reply struct{ OK bool }
		if err := json.Unmarshal(e.HandleJSON([]byte(r.body), r.role), &reply); err != nil || reply.OK {
			t.Fatalf("unauthorized request accepted: %v", r)
		}
	}
}
func TestStorageFailureStopsScheduling(t *testing.T) {
	e := newEngine(t, func(context.Context, []string, string, string, func(Progress)) (string, error) {
		t.Error("must not start without durable state")
		return "", nil
	})
	importTest(t, e, "original")
	p := filepath.Join(e.root, "state.json")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0700); err != nil {
		t.Fatal(err)
	}
	e.online = true
	e.wifi = true
	e.Tick()
	if !e.fault || len(e.active) != 0 {
		t.Fatal("failed-open on write error")
	}
}
func TestCorruptStateDoesNotReset(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "state.json"), []byte("{"), 0600)
	if _, err := Open(d, nil); err == nil {
		t.Fatal("corrupt state silently reset")
	}
}

func TestRemoteLivePhotoComponentIsNotRetried(t *testing.T) {
	e := newEngine(t, func(context.Context, []string, string, string, func(Progress)) (string, error) {
		return "", errRemoteComponentExists
	})
	j := importTest(t, e, "original")
	e.online = true
	e.wifi = true
	e.Tick()
	waitIdle(t, e)
	if j.State != "failed" || j.Error != "remote_live_photo_component_exists" || j.Attempts != 1 {
		t.Fatal("duplicate component outcome lost")
	}
}

func TestStructurallyCorruptStateRejected(t *testing.T) {
	for _, mutate := range []func(*State){
		func(s *State) { s.Jobs = append(s.Jobs, nil) },
		func(s *State) { s.Jobs[0].ID = "../credentials.json" },
		func(s *State) { s.Jobs[0].Resources[0].Name = "../credentials.json" },
		func(s *State) { s.Jobs[0].State = "unknown" },
		func(s *State) { s.Jobs = append(s.Jobs, s.Jobs[0]) },
	} {
		e := newEngine(t, nil)
		importTest(t, e, "original")
		mutate(&e.state)
		if err := e.save(); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(e.root, nil); err == nil {
			t.Fatal("accepted invalid persisted job")
		}
	}
}

func TestEmbeddedBackgroundPauseAndReopen(t *testing.T) {
	started := make(chan struct{})
	e := newEngine(t, func(ctx context.Context, _ []string, _ string, _ string, cb func(Progress)) (string, error) {
		cb(Progress{State: "uploading"})
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	j := importTest(t, e, "original")
	e.HandleJSON([]byte(`{"op":"conditions","online":true,"wifi":true}`), "daemon")
	e.Tick()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not start")
	}
	e.HandleJSON([]byte(`{"op":"conditions","online":false,"wifi":true}`), "daemon")
	waitIdle(t, e)
	if j.State != "pending" || j.Attempts != 0 {
		t.Fatalf("background pause lost queue/retry budget: %+v", j)
	}
	e.Tick()
	waitIdle(t, e) // Would panic by re-closing started if scheduling ignored suspension.
	e.Close()
	next, err := Open(e.root, func(context.Context, []string, string, string, func(Progress)) (string, error) {
		return "committed", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	next.Tick()
	waitIdle(t, next)
	if next.find(j.ID).State != "pending" {
		t.Fatal("reopened queue must await foreground/network conditions")
	}
	next.HandleJSON([]byte(`{"op":"conditions","online":true,"wifi":true}`), "daemon")
	next.Tick()
	waitIdle(t, next)
	if next.find(j.ID).State != "completed" {
		t.Fatal("foreground reopen did not finish queued upload")
	}
}

func TestGooglePhotosSettingsRole(t *testing.T) {
	for _, op := range []string{"configure", "account_add", "account_native", "account_remove", "account_select", "begin", "append", "seal"} {
		if !roleAllowed("googlephotos", op) {
			t.Fatalf("in-app settings/import denied: %s", op)
		}
	}
	if roleAllowed("googlephotos", "conditions") || roleAllowed("photos", "account_add") {
		t.Fatal("expanded role crossed native boundary")
	}
}

func TestOriginalDoesNotReuseLegacyUnverifiedCompletion(t *testing.T) {
	e := newEngine(t, nil)
	old := importTest(t, e, "original")
	old.State = "completed"
	old.MediaKey = "legacy-saver-match"
	fresh := importTest(t, e, "original")
	if fresh.State != "pending" {
		t.Fatal("legacy completion prevented sending original bytes")
	}
	fresh.State = "completed"
	fresh.OriginalPolicy = 1
	fresh.MediaKey = "original-key"
	duplicate := importTest(t, e, "original")
	if duplicate.State != "cancelled" {
		t.Fatal("current original completion was not deduplicated")
	}
}

func TestCancelAllClearsEverything(t *testing.T) {
	started := make(chan struct{})
	e := newEngine(t, func(ctx context.Context, _ []string, _ string, _ string, cb func(Progress)) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	completed := importTest(t, e, "original")
	completed.State = "completed"
	completed.MediaKey = "committed-key"
	history := importTest(t, e, "saver")
	history.State = "cancelled"
	active := importTest(t, e, "quota")
	e.online = true
	e.wifi = true
	e.Tick()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("upload did not start")
	}
	e.mu.Lock()
	_, err := e.handle(Request{Op: "cancel_all"}, "settings")
	e.mu.Unlock()
	waitIdle(t, e)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.state.Jobs) != 0 {
		t.Fatalf("history survived cancel_all: %d jobs", len(e.state.Jobs))
	}
	if e.find(completed.ID) != nil || e.find(history.ID) != nil || e.find(active.ID) != nil {
		t.Fatal("cleared jobs still resolvable")
	}
	for _, j := range []*Job{completed, history, active} {
		if _, err := os.Stat(e.jobDir(j.ID)); !os.IsNotExist(err) {
			t.Fatalf("staging survived cancel_all: %s", j.ID)
		}
	}
	b, _ := os.ReadFile(filepath.Join(e.root, "state.json"))
	if strings.Contains(string(b), completed.ID) || strings.Contains(string(b), active.ID) {
		t.Fatal("cleared jobs persisted")
	}
	// The engine must keep accepting work after the list was emptied.
	next := importTest(t, e, "original")
	if next.State != "pending" {
		t.Fatalf("queue unusable after cancel_all: %s", next.State)
	}
}

func TestCancelAllOnEmptyQueue(t *testing.T) {
	e := newEngine(t, nil)
	e.mu.Lock()
	_, err := e.handle(Request{Op: "cancel_all"}, "settings")
	e.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if len(e.state.Jobs) != 0 {
		t.Fatal("empty queue changed by cancel_all")
	}
}

func TestCancelAllRoleBoundaries(t *testing.T) {
	for _, role := range []string{"settings", "googlephotos", "photos"} {
		if !roleAllowed(role, "cancel_all") {
			t.Fatalf("cancel_all denied for %s", role)
		}
	}
	if roleAllowed("daemon", "cancel_all") {
		t.Fatal("daemon must not clear the queue")
	}
}
