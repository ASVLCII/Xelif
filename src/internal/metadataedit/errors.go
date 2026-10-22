package metadataedit

import (
	"errors"
	"fmt"
)

// Kind classifies failures so callers do not need to parse error strings.
type Kind string

const (
	KindInvalidInput    Kind = "invalid_input"
	KindCanceled        Kind = "canceled"
	KindUnsupported     Kind = "unsupported"
	KindUnsafeFile      Kind = "unsafe_file"
	KindInspect         Kind = "inspect"
	KindJournalExists   Kind = "journal_exists"
	KindJournalIO       Kind = "journal_io"
	KindCorruptJournal  Kind = "corrupt_journal"
	KindJournalVersion  Kind = "journal_version"
	KindAlreadyRestored Kind = "already_restored"
	KindMutation        Kind = "mutation"
	KindRollback        Kind = "rollback"
)

// Error is the typed error returned by this package.
type Error struct {
	Kind Kind
	Op   string
	Path string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	message := string(e.Kind)
	if e.Op != "" {
		message = e.Op + ": " + message
	}
	if e.Path != "" {
		message += " " + fmt.Sprintf("%q", e.Path)
	}
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) Is(target error) bool {
	var other *Error
	return errors.As(target, &other) && e != nil && other.Kind != "" && e.Kind == other.Kind
}

var (
	ErrInvalidInput    = &Error{Kind: KindInvalidInput}
	ErrCanceled        = &Error{Kind: KindCanceled}
	ErrUnsupported     = &Error{Kind: KindUnsupported}
	ErrUnsafeFile      = &Error{Kind: KindUnsafeFile}
	ErrJournalExists   = &Error{Kind: KindJournalExists}
	ErrCorruptJournal  = &Error{Kind: KindCorruptJournal}
	ErrJournalVersion  = &Error{Kind: KindJournalVersion}
	ErrAlreadyRestored = &Error{Kind: KindAlreadyRestored}
)

func newError(kind Kind, op, path string, err error) error {
	return &Error{Kind: kind, Op: op, Path: path, Err: err}
}
