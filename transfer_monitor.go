package kwlib

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// For displaying multiple simultaneous transfers
var transferDisplay struct {
	update_lock sync.RWMutex
	display     int64
	monitors    []*tmon
}

type TMon interface {
	Seek(offset int64, whence int) (int64, error)
	Read(p []byte) (n int, err error)
	Abort()
}

// Add Transfer to transferDisplay.
// Parameters are "name" displayed for file transfer, "limit_sz" for when to pause transfer (aka between calls/chunks), and "total_sz" the total size of the transfer.
func TransferMonitor(name string, total_size int64, source io.ReadSeeker) TMon {
	transferDisplay.update_lock.Lock()
	defer transferDisplay.update_lock.Unlock()

	var short_name []rune

	for i, v := range name {
		if i < 8 {
			short_name = append(short_name, v)
		} else {
			short_name = append(short_name, []rune("...")[0:]...)
			break
		}
	}

	tm := &tmon{
		flag:       trans_active,
		name:       name,
		short_name: string(short_name),
		total_size: total_size,
		transfered: 0,
		offset:     0,
		rate:       "0.0bps",
		start_time: time.Now(),
		source:     source,
	}

	transferDisplay.monitors = append(transferDisplay.monitors, tm)

	if len(transferDisplay.monitors) == 1 {
		PleaseWait.Hide()
		transferDisplay.display = 1

		go func() {
			defer transferDisplay.update_lock.Unlock()
			for {
				transferDisplay.update_lock.Lock()

				var monitors []*tmon

				// Clean up transfers.
				for i := len(transferDisplay.monitors) - 1; i >= 0; i-- {
					if transferDisplay.monitors[i].flag.Has(trans_closed) {
						transferDisplay.monitors = append(transferDisplay.monitors[:i], transferDisplay.monitors[i+1:]...)
					} else {
						monitors = append(monitors, transferDisplay.monitors[i])
					}
				}

				if len(transferDisplay.monitors) == 0 {
					PleaseWait.Show()
					return
				}

				transferDisplay.update_lock.Unlock()

				// Display transfers.
				for _, v := range monitors {
					for i := 0; i < 10; i++ {
						if v.flag.Has(trans_active) {
							v.showTransfer(false)
						} else {
							break
						}
						time.Sleep(time.Millisecond * 200)
					}
				}
			}
		}()

	}

	return tm
}

// Wrapper Seeker
func (tm *tmon) Seek(offset int64, whence int) (int64, error) {
	o, err := tm.source.Seek(offset, whence)
	tm.transfered = o
	tm.offset = o
	return o, err
}

// Wrapped Reader
func (tm *tmon) Read(p []byte) (n int, err error) {
	n, err = tm.source.Read(p)
	atomic.StoreInt64(&tm.transfered, atomic.LoadInt64(&tm.transfered)+int64(n))
	if err != nil {
		if tm.flag.Has(trans_closed) {
			return
		}
		tm.showTransfer(true)
		tm.flag.Set(trans_closed)
	}
	return
}

func (tm *tmon) Abort() {
	if tm.flag.Has(trans_closed) {
		return
	}
	tm.flag.Set(trans_closed)
}

const (
	trans_active = 1 << iota
	trans_closed
	trans_complete
)

// Transfer Monitor
type tmon struct {
	flag       BitFlag
	name       string
	short_name string
	total_size int64
	transfered int64
	offset     int64
	rate       string
	chunk_size int64
	start_time time.Time
	source     io.ReadSeeker
}

// Outputs progress of TMonitor.
func (t *tmon) showTransfer(log bool) {
	transfered := atomic.LoadInt64(&t.transfered)
	rate := t.showRate()

	var (
		output func(vars ...interface{})
		name   string
	)

	if log {
		t.flag.Unset(trans_active)
		name = t.name
		output = Log
	} else {
		name = t.short_name
		output = Flash
	}

	if t.total_size > -1 {
		output("[%s] %s %s (%s/%s)", name, rate, t.progressBar(), HumanSize(transfered), HumanSize(t.total_size))
	} else {
		output("[%s] %s (%s)", t.name, rate, HumanSize(transfered))
	}
}

// Provides average rate of transfer.
func (t *tmon) showRate() string {

	transfered := atomic.LoadInt64(&t.transfered)
	if transfered == 0 || t.flag.Has(trans_complete) {
		return t.rate
	}

	since := time.Since(t.start_time).Seconds()
	if since < 0.1 {
		since = 0.1
	}

	sz := float64(transfered-t.offset) * 8 / since

	names := []string{
		"bps",
		"kbps",
		"mbps",
		"gbps",
	}

	suffix := 0

	for sz >= 1000 && suffix < len(names)-1 {
		sz = sz / 1000
		suffix++
	}

	if sz != 0.0 {
		t.rate = fmt.Sprintf("%.1f%s", sz, names[suffix])
	} else {
		t.rate = "0.0bps"
	}

	if !t.flag.Has(trans_complete) && atomic.LoadInt64(&t.transfered)+t.offset == t.total_size {
		t.flag.Set(trans_complete)
	}

	return t.rate
}

// Produces progress bar for information on update.
func (t *tmon) progressBar() string {
	num := int((float64(atomic.LoadInt64(&t.transfered)) / float64(t.total_size)) * 100)
	if t.total_size == 0 {
		num = 100
	}
	var display [25]rune
	for n := range display {
		if n < num/4 {
			display[n] = '░'
		} else {
			display[n] = '.'
		}
	}
	return fmt.Sprintf("[%s] %d%%", string(display[0:]), int(num))
}
