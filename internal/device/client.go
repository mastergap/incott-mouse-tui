package device

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/mastergap/incott-mouse-tui/internal/protocol"
	"github.com/mastergap/incott-mouse-tui/internal/state"
)

// Options tune timing; zero values select production defaults.
type Options struct {
	RetryInterval time.Duration // wait between connection attempts (default 2s)
	LostInterval  time.Duration // wait after losing a connection (default 1s)
	PollInterval  time.Duration // wait between feature-report polls while reading a setting (default 50ms)
	PollAttempts  int           // feature-report polls per read (default 10)
	ReadSlice     time.Duration // length of one interrupt read; bounds command latency (default 50ms)
	ConfirmWithin time.Duration // how long to wait for an interrupt report to confirm DPI/polling (default 2s)
}

func (o Options) withDefaults() Options {
	if o.RetryInterval == 0 {
		o.RetryInterval = 2 * time.Second
	}
	if o.LostInterval == 0 {
		o.LostInterval = time.Second
	}
	if o.PollInterval == 0 {
		o.PollInterval = 50 * time.Millisecond
	}
	if o.PollAttempts == 0 {
		o.PollAttempts = 10
	}
	if o.ReadSlice == 0 {
		o.ReadSlice = 50 * time.Millisecond
	}
	if o.ConfirmWithin == 0 {
		o.ConfirmWithin = 2 * time.Second
	}
	return o
}

// Client keeps a connection to the mouse and exposes its state and settings.
//
// Run owns the HID handle on a single goroutine. Other goroutines talk to it only through
// Apply/Refresh (request channel) and Events (latest-state channel); no locks are shared.
type Client struct {
	conn   Connector
	opt    Options
	reqs   chan request
	events chan state.State
	stop   chan struct{} // closed when Run returns
	latest atomic.Pointer[state.State]
}

type request struct {
	setting *Setting // nil = refresh
	done    chan error
}

// New creates a client. Call Run in a goroutine.
func New(conn Connector, opt Options) *Client {
	c := &Client{
		conn:   conn,
		opt:    opt.withDefaults(),
		reqs:   make(chan request),
		events: make(chan state.State, 1),
		stop:   make(chan struct{}),
	}
	c.latest.Store(&state.State{})
	return c
}

// Events delivers state snapshots. The channel holds only the latest snapshot: a slow
// consumer skips intermediate states but always sees the newest. It is closed when Run returns.
func (c *Client) Events() <-chan state.State { return c.events }

// State returns the latest snapshot.
func (c *Client) State() state.State { return *c.latest.Load() }

// Apply writes a setting and returns once the mouse has confirmed (or failed). On success the
// new value is in the published state. ErrMismatch means the mouse reports another value;
// the state then holds the value the mouse reports.
func (c *Client) Apply(ctx context.Context, s Setting) error {
	return c.submit(ctx, request{setting: &s, done: make(chan error, 1)})
}

// Refresh re-reads all settings from the mouse.
func (c *Client) Refresh(ctx context.Context) error {
	return c.submit(ctx, request{done: make(chan error, 1)})
}

func (c *Client) submit(ctx context.Context, r request) error {
	select {
	case c.reqs <- r:
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return ErrStopped
	}
	select {
	case err := <-r.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return ErrStopped
	}
}

// Run connects, serves requests and reconnects until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	defer close(c.events)
	defer close(c.stop)

	st := state.State{}
	c.publish(st)
	for ctx.Err() == nil {
		tr, info, err := c.conn.Connect()
		if err != nil {
			slog.Debug("connect failed", "err", err)
			st = state.State{Err: err.Error()}
			if errors.Is(err, ErrPermission) {
				st.Connection = state.PermissionDenied
			}
			c.publish(st)
			if !c.wait(ctx, c.opt.RetryInterval) {
				return
			}
			continue
		}
		slog.Info("connected", "name", info.Name, "pid", fmt.Sprintf("0x%04X", info.ProductID))
		lost := c.session(ctx, tr, info)
		slog.Info("session ended", "reason", lost)
		_ = tr.Close()
		if ctx.Err() != nil {
			return
		}
		st = state.State{Err: lost.Error()}
		c.publish(st)
		if !c.wait(ctx, c.opt.LostInterval) {
			return
		}
	}
}

// wait sleeps for d while answering requests with ErrNotConnected. False if ctx ended.
func (c *Client) wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
			return true
		case r := <-c.reqs:
			r.done <- ErrNotConnected
		}
	}
}

func (c *Client) publish(st state.State) {
	c.latest.Store(&st)
	select { // drop the stale snapshot, if any; this goroutine is the only sender
	case <-c.events:
	default:
	}
	c.events <- st
}

// session runs one connection. It returns why the connection ended (never nil unless ctx ended).
func (c *Client) session(ctx context.Context, tr Transport, info Info) error {
	st := state.State{Connection: state.Connecting, Name: info.Name, ProductID: info.ProductID}
	c.publish(st)

	if err := c.readAll(tr, &st); err != nil {
		return err
	}
	st.Connection = state.Connected
	slog.Info("settings read", "state", fmt.Sprintf("%+v", st))
	c.publish(st)

	buf := make([]byte, 64)
	for ctx.Err() == nil {
		select {
		case r := <-c.reqs:
			err := c.handle(tr, &st, r)
			c.publish(st) // publish before replying: a caller sees the new state as soon as Apply returns
			r.done <- err
			if errors.Is(err, ErrConnectionLost) {
				return err
			}
			continue
		default:
		}

		n, err := tr.ReadInterrupt(buf, int(c.opt.ReadSlice/time.Millisecond))
		if err != nil {
			return fmt.Errorf("%w: %v", ErrConnectionLost, err)
		}
		if n > 0 && protocol.IsInterruptStatus(buf[:n]) {
			s, err := protocol.ParseInterruptStatus(buf[:n])
			if err != nil {
				continue // ignore malformed report
			}
			before := st
			applyStatus(&st, s)
			if st != before {
				c.publish(st)
			}
		}
	}
	return ctx.Err()
}

// readAll reads every setting. Unanswered settings stay unknown; the receiver LED, which
// some units never answer, is flagged unsupported.
func (c *Client) readAll(tr Transport, st *state.State) error {
	for _, k := range []Kind{KindDPI, KindLOD, KindAngleSnap, KindRipple, KindDebounce, KindSleep, KindReceiverLED} {
		err := c.read(tr, st, k)
		switch {
		case errors.Is(err, ErrConnectionLost):
			return err
		case errors.Is(err, ErrTimeout) && k == KindReceiverLED:
			st.ReceiverLEDUnsupported = true
		}
	}
	return nil
}

func (c *Client) handle(tr Transport, st *state.State, r request) error {
	if r.setting == nil {
		// DPI, polling rate and battery come live from interrupt reports; the 0x89 response is a
		// stale snapshot for them (hardware trace, docs/protocol.md), so a refresh keeps them.
		old := *st
		*st = state.State{Connection: state.Connected, Name: st.Name, ProductID: st.ProductID, LOD07Unsupported: st.LOD07Unsupported}
		err := c.readAll(tr, st)
		if old.DPI.OK {
			st.Battery, st.Charging, st.DPI, st.Polling = old.Battery, old.Charging, old.DPI, old.Polling
		}
		return err
	}
	return c.apply(tr, st, *r.setting)
}

func (c *Client) apply(tr Transport, st *state.State, s Setting) error {
	if s.Kind == KindReceiverLED && st.ReceiverLEDUnsupported {
		return ErrUnsupported
	}
	slog.Debug("apply", "setting", s.String())
	rep, err := s.report()
	if err != nil {
		return err
	}
	if err := tr.SendFeature(rep[:]); err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionLost, err)
	}
	if s.Kind == KindDPI || s.Kind == KindPolling {
		return c.confirmFromInterrupt(tr, st, s)
	}
	switch err := c.read(tr, st, s.Kind); {
	case errors.Is(err, ErrConnectionLost):
		return err
	case err != nil:
		// The write succeeded but the read-back failed: trust the write.
		assume(st, s.Kind, s.Value)
		return nil
	}
	if got, ok := value(st, s.Kind); ok && got != s.Value {
		if s.Kind == KindLOD && s.Value == protocol.LOD07mm {
			st.LOD07Unsupported = true // hardware: some units silently ignore 0.7 mm
		}
		return fmt.Errorf("%w: %s requested %d, device reports %d", ErrMismatch, s.Kind, s.Value, got)
	}
	return nil
}

// confirmFromInterrupt waits for the status report the mouse pushes after a DPI or polling
// change and checks it carries the requested value. The 0x89 feature response cannot be used:
// it keeps returning the value from before the change.
func (c *Client) confirmFromInterrupt(tr Transport, st *state.State, s Setting) error {
	deadline := time.Now().Add(c.opt.ConfirmWithin)
	buf := make([]byte, 64)
	for time.Now().Before(deadline) {
		n, err := tr.ReadInterrupt(buf, int(c.opt.ReadSlice/time.Millisecond))
		if err != nil {
			return fmt.Errorf("%w: %v", ErrConnectionLost, err)
		}
		if n == 0 || !protocol.IsInterruptStatus(buf[:n]) {
			continue
		}
		rep, err := protocol.ParseInterruptStatus(buf[:n])
		if err != nil {
			continue
		}
		applyStatus(st, rep)
		if got, _ := value(st, s.Kind); got == s.Value {
			return nil
		}
		// An older report (sent before the mouse handled the write) may still be in flight:
		// keep waiting until the deadline.
	}
	got, _ := value(st, s.Kind)
	if got == s.Value {
		return nil // already at the requested value: the mouse sends no report for a no-op
	}
	return fmt.Errorf("%w: %s requested %d, mouse reports %d", ErrMismatch, s.Kind, s.Value, got)
}

// read requests the setting k and stores the response in st. Errors: ErrConnectionLost
// (send failed), ErrTimeout (no usable response), or a parse error.
func (c *Client) read(tr Transport, st *state.State, k Kind) error {
	req := k.request()
	if err := tr.SendFeature(req.Report[:]); err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionLost, err)
	}
	buf := make([]byte, 64)
	for i := 0; i < c.opt.PollAttempts; i++ {
		time.Sleep(c.opt.PollInterval)
		buf[0] = protocol.ReportID
		if _, err := tr.GetFeature(buf); err != nil || !req.Matches(buf) {
			continue
		}
		err := store(st, k, buf)
		if errors.Is(err, protocol.ErrNotReady) {
			continue
		}
		return err
	}
	return ErrTimeout
}
