package machine

import (
	"errors"
	"os"
	"testing"

	"github.com/skevetter/devpod/pkg/pty"
)

func TestHasInteractiveTerminalRequiresStdinAndStdout(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	defer func() { _ = stdinReader.Close() }()
	defer func() { _ = stdinWriter.Close() }()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() { _ = stdoutReader.Close() }()
	defer func() { _ = stdoutWriter.Close() }()

	oldIsTerminalFunc := isTerminalFunc
	t.Cleanup(func() {
		isTerminalFunc = oldIsTerminalFunc
	})

	isTerminalFunc = func(fd uintptr) bool {
		return fd == stdinReader.Fd()
	}
	if hasInteractiveTerminal(stdinReader, stdoutWriter) {
		t.Fatal("expected missing stdout terminal to disable PTY")
	}

	isTerminalFunc = func(fd uintptr) bool {
		return fd == stdinReader.Fd() || fd == stdoutWriter.Fd()
	}
	if !hasInteractiveTerminal(stdinReader, stdoutWriter) {
		t.Fatal("expected PTY when both stdin and stdout are terminals")
	}
}

func TestMakeRawTermUsesInputAndOutputTerminalState(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	defer func() { _ = stdinReader.Close() }()
	defer func() { _ = stdinWriter.Close() }()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() { _ = stdoutReader.Close() }()
	defer func() { _ = stdoutWriter.Close() }()

	stdinState := &pty.TerminalState{}
	stdoutState := &pty.TerminalState{}
	var calls []string

	oldMakeInputRawTerm := makeInputRawTerm
	oldMakeOutputRawTerm := makeOutputRawTerm
	oldRestoreTerminalFunc := restoreTerminalFunc
	t.Cleanup(func() {
		makeInputRawTerm = oldMakeInputRawTerm
		makeOutputRawTerm = oldMakeOutputRawTerm
		restoreTerminalFunc = oldRestoreTerminalFunc
	})

	makeInputRawTerm = func(fd uintptr) (*pty.TerminalState, error) {
		calls = append(calls, "input")
		if fd != stdinReader.Fd() {
			t.Fatalf("expected stdin fd %d, got %d", stdinReader.Fd(), fd)
		}
		return stdinState, nil
	}
	makeOutputRawTerm = func(fd uintptr) (*pty.TerminalState, error) {
		calls = append(calls, "output")
		if fd != stdoutWriter.Fd() {
			t.Fatalf("expected stdout fd %d, got %d", stdoutWriter.Fd(), fd)
		}
		return stdoutState, nil
	}
	restoreTerminalFunc = func(fd uintptr, state *pty.TerminalState) error {
		switch {
		case fd == stdoutWriter.Fd() && state == stdoutState:
			calls = append(calls, "restore-output")
		case fd == stdinReader.Fd() && state == stdinState:
			calls = append(calls, "restore-input")
		default:
			t.Fatalf("unexpected restore call: fd=%d state=%p", fd, state)
		}
		return nil
	}

	restore, err := makeRawTerm(stdinReader, stdoutWriter)
	if err != nil {
		t.Fatalf("make raw term: %v", err)
	}
	restore()

	want := []string{"input", "output", "restore-output", "restore-input"}
	if len(calls) != len(want) {
		t.Fatalf("unexpected calls: got %v want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("unexpected calls: got %v want %v", calls, want)
		}
	}
}

func TestMakeRawTermRestoresInputOnOutputFailure(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	defer func() { _ = stdinReader.Close() }()
	defer func() { _ = stdinWriter.Close() }()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() { _ = stdoutReader.Close() }()
	defer func() { _ = stdoutWriter.Close() }()

	stdinState := &pty.TerminalState{}
	outputErr := errors.New("output raw failed")
	var restoreCalled bool

	oldMakeInputRawTerm := makeInputRawTerm
	oldMakeOutputRawTerm := makeOutputRawTerm
	oldRestoreTerminalFunc := restoreTerminalFunc
	t.Cleanup(func() {
		makeInputRawTerm = oldMakeInputRawTerm
		makeOutputRawTerm = oldMakeOutputRawTerm
		restoreTerminalFunc = oldRestoreTerminalFunc
	})

	makeInputRawTerm = func(fd uintptr) (*pty.TerminalState, error) {
		if fd != stdinReader.Fd() {
			t.Fatalf("expected stdin fd %d, got %d", stdinReader.Fd(), fd)
		}
		return stdinState, nil
	}
	makeOutputRawTerm = func(fd uintptr) (*pty.TerminalState, error) {
		if fd != stdoutWriter.Fd() {
			t.Fatalf("expected stdout fd %d, got %d", stdoutWriter.Fd(), fd)
		}
		return nil, outputErr
	}
	restoreTerminalFunc = func(fd uintptr, state *pty.TerminalState) error {
		if fd != stdinReader.Fd() || state != stdinState {
			t.Fatalf("unexpected restore call: fd=%d state=%p", fd, state)
		}
		restoreCalled = true
		return nil
	}

	restore, err := makeRawTerm(stdinReader, stdoutWriter)
	if !errors.Is(err, outputErr) {
		t.Fatalf("expected output error, got %v", err)
	}
	if restore == nil {
		t.Fatal("expected restore function")
	}
	if !restoreCalled {
		t.Fatal("expected stdin terminal state to be restored after output setup failure")
	}
}

func TestMakeRawTermStopsWhenInputSetupFails(t *testing.T) {
	stdinReader, stdinWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	defer func() { _ = stdinReader.Close() }()
	defer func() { _ = stdinWriter.Close() }()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer func() { _ = stdoutReader.Close() }()
	defer func() { _ = stdoutWriter.Close() }()

	inputErr := errors.New("input raw failed")
	var outputCalled bool
	var restoreCalled bool

	oldMakeInputRawTerm := makeInputRawTerm
	oldMakeOutputRawTerm := makeOutputRawTerm
	oldRestoreTerminalFunc := restoreTerminalFunc
	t.Cleanup(func() {
		makeInputRawTerm = oldMakeInputRawTerm
		makeOutputRawTerm = oldMakeOutputRawTerm
		restoreTerminalFunc = oldRestoreTerminalFunc
	})

	makeInputRawTerm = func(fd uintptr) (*pty.TerminalState, error) {
		if fd != stdinReader.Fd() {
			t.Fatalf("expected stdin fd %d, got %d", stdinReader.Fd(), fd)
		}
		return nil, inputErr
	}
	makeOutputRawTerm = func(uintptr) (*pty.TerminalState, error) {
		outputCalled = true
		return &pty.TerminalState{}, nil
	}
	restoreTerminalFunc = func(uintptr, *pty.TerminalState) error {
		restoreCalled = true
		return nil
	}

	restore, err := makeRawTerm(stdinReader, stdoutWriter)
	if !errors.Is(err, inputErr) {
		t.Fatalf("expected input error, got %v", err)
	}
	if restore == nil {
		t.Fatal("expected restore function")
	}
	if outputCalled {
		t.Fatal("did not expect output raw setup after input failure")
	}
	if restoreCalled {
		t.Fatal("did not expect terminal restore after input failure")
	}
}
