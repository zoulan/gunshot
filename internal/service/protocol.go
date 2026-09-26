package service

import (
	"encoding/json"
	"errors"
	"os"
)

// Native code supplies identity from the kernel audit trailer, never JSON.
func roleAllowed(role, op string) bool {
	if role == "daemon" {
		return op == "conditions"
	}
	common := op == "upload_summary" || op == "job" || op == "ping" || op == "list" || op == "accounts" || op == "options" || op == "retry" || op == "cancel" || op == "cancel_all" || op == "clear_completed" || op == "retry_failed"
	if role == "settings" || role == "googlephotos" {
		return common || (role == "googlephotos" && (op == "begin" || op == "append" || op == "seal" || op == "account_native" || op == "native_bearer" || op == "native_bearer_clear")) || op == "configure" || op == "account_add" || op == "account_remove" || op == "account_select"
	}
	if role == "photos" {
		return common || op == "begin" || op == "append" || op == "seal"
	}
	return false
}
func (e *Engine) HandleJSON(b []byte, role string) []byte {
	if len(b) > MaxMessage {
		return response(nil, errRequest)
	}
	var r Request
	if json.Unmarshal(b, &r) != nil || !roleAllowed(role, r.Op) {
		return response(nil, errors.New("unauthorized or invalid request"))
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.fault {
		return response(nil, errors.New("storage error; restart after checking free space"))
	}
	data, err := e.handle(r, role)
	return response(data, err)
}
func response(data any, err error) []byte {
	var v any
	if err != nil {
		v = map[string]any{"ok": false, "error": "operation_failed"}
	} else {
		v = map[string]any{"ok": true, "data": data}
	}
	b, _ := json.Marshal(v)
	if len(b) > MaxMessage {
		return []byte(`{"ok":false,"error":"response_too_large"}`)
	}
	return b
}
func (e *Engine) handle(r Request, role string) (any, error) {
	switch r.Op {
	case "ping":
		return map[string]any{"version": 1}, nil
	case "options":
		return e.state.Options, nil
	case "upload_summary":
		return e.uploadSummary(), nil
	case "conditions":
		e.online = r.Online
		e.wifi = r.WiFi
		e.charging = r.Charging
		if !e.online || (e.state.Options.WiFiOnly && !e.wifi) || (e.state.Options.ChargingOnly && !e.charging) {
			for _, c := range e.active {
				c()
			}
		}
		return nil, nil
	case "configure":
		if r.Options == nil || !r.Options.valid() {
			return nil, errRequest
		}
		changed := e.state.Options != *r.Options
		e.state.Options = *r.Options
		if r.Options.Paused || (r.Options.WiFiOnly && !e.wifi) || (r.Options.ChargingOnly && !e.charging) {
			for _, c := range e.active {
				c()
			}
		}
		if !changed {
			return nil, nil
		}
		return nil, e.save()
	case "list":
		start := r.Cursor
		if start < 0 || start > len(e.state.Jobs) {
			return nil, errRequest
		}
		end := min(start+25, len(e.state.Jobs))
		next := -1
		if end < len(e.state.Jobs) {
			next = end
		}
		return map[string]any{"jobs": e.state.Jobs[start:end], "next": next, "online": e.online, "wifi": e.wifi, "charging": e.charging}, nil
	case "accounts", "account_native", "native_bearer", "native_bearer_clear", "account_add", "account_remove", "account_select":
		return e.accounts(r)
	case "begin":
		if !accountExists(r.Account) {
			return nil, errRequest
		}
		return e.begin(r, role)
	case "clear_completed":
		next := e.state.Jobs[:0]
		for _, j := range e.state.Jobs {
			if j.State == "completed" || j.State == "cancelled" {
				delete(e.jobsByID, j.ID)
			} else {
				next = append(next, j)
			}
		}
		if len(next) == len(e.state.Jobs) {
			return nil, nil
		}
		clear(e.state.Jobs[len(next):]) // Release removed jobs held by the backing array.
		e.state.Jobs = next
		return nil, e.save()
	case "retry_failed":
		changed := false
		for _, j := range e.state.Jobs {
			if j.State == "failed" {
				j.resetRetry()
				changed = true
			}
		}
		if !changed {
			return nil, nil
		}
		return nil, e.save()
	case "cancel_all":
		// One durable step: cancel everything unfinished, then empty the whole
		// list. Media already committed to Google Photos is never touched; only
		// local staging directories are discarded.
		for _, j := range e.state.Jobs {
			if j.State != "completed" && j.State != "cancelled" {
				j.CancelRequested = true
				if c := e.active[j.ID]; c != nil {
					c()
				} else {
					j.State = "cancelled"
				}
			}
			delete(e.jobsByID, j.ID)
			delete(e.importHashes, j.ID)
		}
		if len(e.state.Jobs) == 0 {
			return nil, nil
		}
		removed := e.state.Jobs
		e.state.Jobs = nil
		for _, j := range removed {
			_ = os.RemoveAll(e.jobDir(j.ID))
		}
		clear(removed) // Release job pointers held by the backing array.
		return nil, e.save()
	}
	if !validID(r.ID) {
		return nil, errRequest
	}
	j := e.find(r.ID)
	if j == nil {
		return nil, errRequest
	}
	if (r.Op == "append" || r.Op == "seal") && j.Owner != role {
		return nil, errRequest
	}
	switch r.Op {
	case "job":
		return j, nil
	case "append":
		return nil, e.appendChunk(j, r)
	case "seal":
		return e.seal(j)
	case "cancel":
		if j.State == "completed" || j.State == "cancelled" {
			return nil, errRequest
		}
		j.CancelRequested = true
		if c := e.active[j.ID]; c != nil {
			c()
		} else {
			j.State = "cancelled"
			delete(e.importHashes, j.ID)
		}
		err := e.save()
		if err == nil && j.State == "cancelled" {
			_ = os.RemoveAll(e.jobDir(j.ID))
		}
		return nil, err
	case "retry":
		if j.State != "failed" {
			return nil, errRequest
		}
		j.resetRetry()
		return nil, e.save()
	}
	return nil, errRequest
}
