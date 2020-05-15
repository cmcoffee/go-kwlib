/*
	go-kwlib is a library built around creating kiteworks applications in golang.
	It implements common functions for automating tasks.
*/

package kwlib

import (
	"crypto/rand"
	"fmt"
	"github.com/cmcoffee/go-ask"
	"github.com/cmcoffee/go-kvliter"
	"github.com/cmcoffee/go-nfo"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

const (
	NONE  = ""
	SLASH = string(os.PathSeparator)
)

func init() {
	nfo.SetTimestamp(nfo.AUX, false)
}

// Import from go-nfo.
var (
	Log        = nfo.Log
	Fatal      = nfo.Fatal
	Notice     = nfo.Notice
	Flash      = nfo.Flash
	Stdout     = nfo.Stdout
	Warn       = nfo.Warn
	Defer      = nfo.Defer
	Debug      = nfo.Debug
	Snoop      = nfo.Aux
	ForSecret  = ask.ForSecret
	ForInput   = ask.ForInput
	Exit       = nfo.Exit
	PleaseWait = nfo.PleaseWait
	Stderr     = nfo.Stderr
	Confirm    = nfo.Confirm
	HideTS     = nfo.HideTS
	ShowTS     = nfo.ShowTS
)

// Atomic BitFlag
type BitFlag int64

func (B *BitFlag) Has(flag int) bool {
	if atomic.LoadInt64((*int64)(B))&int64(flag) != 0 {
		return true
	}
	return false
}

// Set BitFlag
func (B *BitFlag) Set(flag int) {
	atomic.StoreInt64((*int64)(B), atomic.LoadInt64((*int64)(B))|int64(flag))
}

// Unset BitFlag
func (B *BitFlag) Unset(flag int) {
	atomic.StoreInt64((*int64)(B), atomic.LoadInt64((*int64)(B))&^int64(flag))
}

// Enable Debug Logging Output
func EnableDebug() {
	nfo.SetOutput(nfo.DEBUG, os.Stdout)
	nfo.LogFileAppend(nfo.ERROR, nfo.DEBUG)
}

// Disables Flash from being displayed.
func Quiet() {
	Flash = func(vars ...interface{}) { return }
}

var error_counter uint32

// Returns amount of times Err has been triggered.
func ErrorCount() uint32 {
	return atomic.LoadUint32(&error_counter)
}

func Err(input ...interface{}) {
	atomic.AddUint32(&error_counter, 1)
	nfo.Err(input...)
}

var Path = filepath.Clean

// Wrapper around go-kvlite.
type Database struct {
	db kvliter.Store
}

// Opens go-kvlite sqlite database.
func OpenDatabase(file string, padlock ...byte) (*Database, error) {
	db, err := kvliter.Open(file, padlock...)
	if err != nil {
		return nil, err
	}
	return &Database{db}, nil
}

// Opens go-kvlite database using mac address for lock.
func SecureDatabase(file string) (*Database, error) {
	// Provides us the mac address of the first interface.
	get_mac_addr := func() []byte {
		ifaces, err := net.Interfaces()
		Critical(err)

		for _, v := range ifaces {
			if len(v.HardwareAddr) == 0 {
				continue
			}
			return v.HardwareAddr
		}
		return nil
	}

	db, err := kvliter.Open(file, get_mac_addr()[0:]...)
	if err != nil {
		if err == kvliter.ErrBadPadlock {
			Notice("Hardware changes detected, you will need to reauthenticate.")
			if err := kvliter.CryptReset(file); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		db, err = kvliter.Open(file, get_mac_addr()[0:]...)
		if err != nil {
			return nil, err
		}
	}
	return &Database{db}, nil
}

// Open a memory-only go-kvlite store.
func OpenCache() *Database {
	db := kvliter.MemStore()
	return &Database{db}
}

// DB Wrappers to perform fatal error checks on each call.
func (d Database) Drop(table string) {
	Critical(d.db.Drop(table))
}

// Encrypt value to go-kvlie, fatal on error.
func (d Database) CryptSet(table, key string, value interface{}) {
	Critical(d.db.CryptSet(table, key, value))
}

// Save value to go-kvlite.
func (d Database) Set(table, key string, value interface{}) {
	Critical(d.db.Set(table, key, value))
}

// Retrieve value from go-kvlite.
func (d Database) Get(table, key string, output interface{}) bool {
	found, err := d.db.Get(table, key, output)
	Critical(err)
	return found
}

// List keys in go-kvlite.
func (d Database) Keys(table string) []string {
	keylist, err := d.db.Keys(table)
	Critical(err)
	return keylist
}

// Count keys in table.
func (d Database) CountKeys(table string) int {
	count, err := d.db.CountKeys(table)
	Critical(err)
	return count
}

// List Tables in DB
func (d Database) Tables() []string {
	tables, err := d.db.Tables()
	Critical(err)
	return tables
}

// Delete value from go-kvlite.
func (d Database) Unset(table, key string) {
	Critical(d.db.Unset(table, key))
}

// Closes go-kvlite database.
func (d Database) Close() {
	Critical(d.db.Close())
}

// Fatal Error Check
func Critical(err error) {
	if err != nil {
		Fatal(err)
	}
}

// Generates a random byte slice of length specified.
func RandBytes(sz int) []byte {
	if sz <= 0 {
		sz = 16
	}

	ch := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/"
	chlen := len(ch)

	rand_string := make([]byte, sz)
	rand.Read(rand_string)

	for i, v := range rand_string {
		rand_string[i] = ch[v%byte(chlen)]
	}
	return rand_string
}

// Error handler for const errors.
type Error string

func (e Error) Error() string { return string(e) }

// Creates folders.
func MkDir(name ...string) (err error) {
	for _, path := range name {
		subs := strings.Split(path, string(os.PathSeparator))
		for i := 0; i < len(subs); i++ {
			p := strings.Join(subs[0:i], string(os.PathSeparator))
			if p == "" {
				p = "."
			}
			_, err = os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					err = os.Mkdir(p, 0766)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	}
	return nil
}

// Parse Timestamps from kiteworks
func ReadKWTime(input string) (time.Time, error) {
	input = strings.Replace(input, "+0000", "Z", 1)
	return time.Parse(time.RFC3339, input)
}

// Write timestamps for kiteworks.
func WriteKWTime(input time.Time) string {
	t := input.UTC().Format(time.RFC3339)
	return strings.Replace(t, "Z", "+0000", 1)
}

// Create standard date YY-MM-DD out of time.Time.
func DateString(input time.Time) string {
	pad := func(num int) string {
		if num < 10 {
			return fmt.Sprintf("0%d", num)
		}
		return fmt.Sprintf("%d", num)
	}

	due_time := input.AddDate(0, 0, -1)
	return fmt.Sprintf("%s-%s-%s", pad(due_time.Year()), pad(int(due_time.Month())), pad(due_time.Day()))
}

// Provides human readable file sizes.
func HumanSize(bytes int64) string {

	names := []string{
		"Bytes",
		"KB",
		"MB",
		"GB",
	}

	suffix := 0
	size := float64(bytes)

	for size >= 1000 && suffix < len(names)-1 {
		size = size / 1000
		suffix++
	}

	return fmt.Sprintf("%.1f%s", size, names[suffix])
}
