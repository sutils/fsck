package main

/*
#cgo darwin CFLAGS: -I/usr/local/include
#cgo darwin LDFLAGS: -L/usr/local/lib
#cgo LDFLAGS: -lreadline -lhistory
#include <stdio.h>
#include <stdlib.h>
#include <readline/readline.h>
#include <readline/history.h>
extern char *_completion_function(char *s, int i);
extern int cgoOnEsc(int count,int key);
extern int cgoOnTerm(int count,int key);
static char *_completion_function_trans(const char *s, int i) {
	return _completion_function((char *) s, i);
}
static void register_readline() {
	rl_completion_entry_function = _completion_function_trans;
	using_history();
}
static void rl_init(){
	register_readline();
	rl_bind_key(27,cgoOnEsc);
	rl_bind_key(3,cgoOnTerm);
}
*/
import "C"

import (
	"io"
	"os/user"
	"path/filepath"
	"regexp"
	"syscall"
	"unsafe"

	"github.com/Centny/gwf/log"
)

// The prompt used by Reader(). The prompt can contain ANSI escape
// sequences, they will be escaped as necessary.
var Prompt = "> "

// The continue prompt used by Reader(). The prompt can contain ANSI escape
// sequences, they will be escaped as necessary.
var Continue = ".."

const (
	promptStartIgnore = string(C.RL_PROMPT_START_IGNORE)
	promptEndIgnore   = string(C.RL_PROMPT_END_IGNORE)
)

// If CompletionAppendChar is non-zero, readline will append the
// corresponding character to the prompt after each completion. A
// typical value would be a space.
var CompletionAppendChar = 0

type state byte

const (
	readerStart state = iota
	readerContinue
	readerEnd
)

type reader struct {
	buf   []byte
	state state
}

var shortEscRegex = "\x1b[@-Z\\-_]"
var csiPrefix = "(\x1b[[]|\xC2\x9b)"
var csiParam = "([0-9]+|\"[^\"]*\")"
var csiSuffix = "[@-~]"
var csiRegex = csiPrefix + "(" + csiParam + "(;" + csiParam + ")*)?" + csiSuffix
var escapeSeq = regexp.MustCompile(shortEscRegex + "|" + csiRegex)

// Begin reading lines. If more than one line is required, the continue prompt
// is used for subsequent lines.
func NewReader() io.Reader {
	return new(reader)
}

func (r *reader) getLine() error {
	prompt := Prompt
	if r.state == readerContinue {
		prompt = Continue
	}
	s, err := String(prompt)
	if err != nil {
		return err
	}
	r.buf = []byte(s)
	return nil
}

func (r *reader) Read(buf []byte) (int, error) {
	if r.state == readerEnd {
		return 0, io.EOF
	}
	if len(r.buf) == 0 {
		err := r.getLine()
		if err == io.EOF {
			r.state = readerEnd
		}
		if err != nil {
			return 0, err
		}
		r.state = readerContinue
	}
	copy(buf, r.buf)
	l := len(buf)
	if len(buf) > len(r.buf) {
		l = len(r.buf)
	}
	r.buf = r.buf[l:]
	return l, nil
}

// Read a line with the given prompt. The prompt can contain ANSI
// escape sequences, they will be escaped as necessary.
func String(prompt string) (string, error) {
	prompt = "\x1b[0m" + prompt // Prepend a 'reset' ANSI escape sequence
	prompt = escapeSeq.ReplaceAllString(prompt, promptStartIgnore+"$0"+promptEndIgnore)
	p := C.CString(prompt)
	rp := C.readline(p)
	s := C.GoString(rp)
	C.free(unsafe.Pointer(p))
	if rp != nil {
		C.free(unsafe.Pointer(rp))
		return s, nil
	}
	return s, io.EOF
}

var callOnKey func(int) int

func StringCallback(prompt string, onkey func(key int) int) (string, error) {
	callOnKey = onkey
	defer func() {
		callOnKey = nil
	}()
	return String(prompt)
}

func rlDone() {
	C.rl_done = 1
}

//export cgoOnEsc
func cgoOnEsc(count, key C.int) (reply C.int) {
	if callOnKey != nil {
		reply = C.int(callOnKey(27))
	}
	rlDone()
	return
}

//export cgoOnTerm
func cgoOnTerm(count, key C.int) (reply C.int) {
	if callOnKey != nil {
		reply = C.int(callOnKey(3))
	}
	rlDone()
	return
}

// This function provides entries for the tab completer.
var Completer = func(query, ctx string) []string {
	return nil
}

var entries []*C.char

// This function can be assigned to the Completer variable to use
// readline's default filename completion, or it can be called by a
// custom completer function to get a list of files and filter it.
func FilenameCompleter(query, ctx string) []string {
	var compls []string
	var c *C.char
	q := C.CString(query)

	for i := 0; ; i++ {
		if c = C.rl_filename_completion_function(q, C.int(i)); c == nil {
			break
		}
		compls = append(compls, C.GoString(c))
		C.free(unsafe.Pointer(c))
	}

	C.free(unsafe.Pointer(q))

	return compls
}

//export _completion_function
func _completion_function(p *C.char, _i C.int) *C.char {
	C.rl_completion_append_character = C.int(CompletionAppendChar)
	i := int(_i)
	if i == 0 {
		es := Completer(C.GoString(p), C.GoString(C.rl_line_buffer))
		entries = make([]*C.char, len(es))
		for i, x := range es {
			entries[i] = C.CString(x)
		}
	}
	if i >= len(entries) {
		return nil
	}
	return entries[i]
}

func SetWordBreaks(cs string) {
	C.rl_completer_word_break_characters = C.CString(cs)
}

// Add an item to the history.
func AddHistory(s string) {
	n := HistorySize()
	if n == 0 || s != GetHistory(n-1) {
		C.add_history(C.CString(s))
	}
}

// Retrieve a line from the history.
func GetHistory(i int) string {
	e := C.history_get(C.int(i + 1))
	if e == nil {
		return ""
	}
	return C.GoString(e.line)
}

// Clear the screen
func ClearScreen() {
	var x, y C.int = 0, 0
	C.rl_clear_screen(x, y)
}

// rl_forced_update_display / redraw
func ForceUpdateDisplay() {
	C.rl_forced_update_display()
}

// Replace current line
func ReplaceLine(text string, clearUndo int) {
	C.rl_replace_line(C.CString(text), C.int(clearUndo))
}

// Redraw current line
func RefreshLine() {
	var x, y C.int = 0, 0
	C.rl_refresh_line(x, y)
}

// Deletes all the items in the history.
func ClearHistory() {
	C.clear_history()
}

// Returns the number of items in the history.
func HistorySize() int {
	return int(C.history_length)
}

// Load the history from a file.
func LoadHistory(path string) error {
	p := C.CString(path)
	e := C.read_history(p)
	C.free(unsafe.Pointer(p))

	if e == 0 {
		return nil
	}
	return syscall.Errno(e)
}

// Save the history to a file.
func SaveHistory(path string) error {
	p := C.CString(path)
	e := C.write_history(p)
	C.free(unsafe.Pointer(p))

	if e == 0 {
		return nil
	}
	return syscall.Errno(e)
}

// Cleanup() frees internal memory and restores terminal
// attributes. This function should be called when program execution
// stops before the return of a String() call, so as not to leave the
// terminal in a corrupted state.
func Cleanup() {
	C.rl_free_line_state()
	C.rl_cleanup_after_signal()
}

func init() {
	C.rl_catch_signals = 0
	C.rl_catch_sigwinch = 0
	C.rl_init()
}

var HISTORY = ""

func SetHistory() {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	HISTORY = filepath.Join(usr.HomeDir, ".fsck_history")
}

func StoreHistory(line string) {
	AddHistory(line)
	err := SaveHistory(HISTORY)
	if err != nil {
		log.E("save history to %v fail with %v", HISTORY, err)
	}
}

func SyncHistory() {
	SetHistory()
	LoadHistory(HISTORY)
}

func ResizeTerminal() {
	C.rl_resize_terminal()
}
