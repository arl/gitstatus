// Package gitstatus provides information about the git status of a working
// tree.
package gitstatus

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"strconv"
	"strings"
)

// Status represents the status of a Git working tree directory.
type Status struct {
	Porcelain

	// NumStashed is the number of stash entries.
	NumStashed int

	// HEAD is the shortened SHA1 of current commit (empty in initial state).
	HEAD string

	// State indicates the state of the working tree.
	State TreeState

	// IsClean reports whether the working tree is in a clean state (i.e empty
	// staging area, no conflicts and no untracked files).
	IsClean bool

	// Insertions is the count of inserted lines in the staging area.
	Insertions int

	// Deletions is the count of deleted lines in the staging area.
	Deletions int
}

// Porcelain holds the Git status variables extracted from calling git status --porcelain.
type Porcelain struct {
	// NumModified is the number of modified files.
	NumModified int

	// NumConflicts is the number of unmerged files.
	NumConflicts int

	// NumUntracked is the number of untracked files.
	NumUntracked int

	// NumStaged is the number of staged files.
	NumStaged int

	// IsDetached reports whether HEAD is not associated to any branch
	// (detached).
	IsDetached bool

	// IsInitial reports whether the working tree is in its initial state (no
	// commit have been performed yet).
	IsInitial bool

	// LocalBranch is the name of the local branch.
	LocalBranch string

	// RemoteBranch is the name of upstream remote branch (tracking).
	RemoteBranch string

	// AheadCount reports by how many commits the local branch is ahead of its upstream branch.
	AheadCount int

	// BehindCount reports by how many commits the local branch is behind its upstream branch.
	BehindCount int
}

var (
	errParseAheadBehind = errors.New("can't parse ahead/behind count")
	errUnexpectedHeader = errors.New("unexpected header format")
)

// New returns the Git Status of the current working directory.
func New() (*Status, error) { return newStatus(context.Background()) }

// NewWithContext is likes New but includes a context.
//
// The provided context is used to stop retrieving git status if the context
// becomes done before all calls to git have completed.
func NewWithContext(ctx context.Context) (*Status, error) { return newStatus(ctx) }

func newStatus(ctx context.Context) (*Status, error) {
	por := Porcelain{}
	err := runAndParse(ctx, &por, "git", "status", "--porcelain=v1", "--branch", "-z")
	if err != nil {
		return nil, err
	}

	stats := stats{}
	err = runAndParse(ctx, &stats, "git", "diff", "--shortstat")
	if err != nil {
		return nil, err
	}

	// All successive commands require at least one commit.
	if por.IsInitial {
		return &Status{Porcelain: por}, nil
	}

	// Count stash entries.
	nstashed := linecount(0)
	if err = runAndParse(ctx, &nstashed, "git", "stash", "list"); err != nil {
		return nil, err
	}

	// Sets other special flags and fields.
	var lines lines
	err = runAndParse(ctx, &lines, "git", "rev-parse", "--git-dir", "--short", "HEAD")
	if err != nil || len(lines) != 2 {
		return nil, err
	}

	isClean := por.NumStaged+por.NumConflicts+por.NumModified+por.NumUntracked == 0

	st := &Status{
		Porcelain:  por,
		State:      treeStateFromDir(strings.TrimSpace(lines[0])),
		HEAD:       strings.TrimSpace(lines[1]),
		NumStashed: int(nstashed),
		IsClean:    isClean,
		Insertions: stats.insertions,
		Deletions:  stats.deletions,
	}

	return st, nil
}

// scanNilBytes is a bufio.SplitFunc function used to tokenize the input with
// nil bytes. The last byte should always be a nil byte or scanNilBytes returns
// an error.
func scanNilBytes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, 0); i >= 0 {
		// We have a full nil-terminated line.
		return i + 1, data[0:i], nil
	}

	// If we're at EOF, we would have a final not ending with a nil byte, we
	// won't allow that.
	if atEOF {
		return 0, nil, errors.New("last line doesn't end with a nil byte")
	}
	// Request more data.
	return 0, nil, nil
}

var fileStatusRx = regexp.MustCompile(`^(##|[ MADRCUT?!]{2}) .*$`)

// parseStatus parses porcelain status and fills it with r.
func (p *Porcelain) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(scanNilBytes)

	var err error
	for scan.Scan() {
		line := scan.Text()
		if !fileStatusRx.MatchString(line) {
			continue
		}

		first, second := line[0], line[1]

		switch {
		case first == '#' && second == '#':
			err = p.parseHeader(line)
		case first == 'U', second == 'U',
			first == 'A' && second == 'A':
			p.NumConflicts++
		case first == 'A' && second == 'M',
			first == 'M' && second == 'M',
			first == 'M' && second == 'D',
			first == 'R' && second == 'M',
			first == 'R' && second == 'D',
			first == 'A' && second == 'T':
			p.NumModified++
			p.NumStaged++
		case second == 'M', second == 'D':
			p.NumModified++
		case first == '?' && second == '?':
			p.NumUntracked++
		default:
			p.NumStaged++
		}

		if err != nil {
			return err
		}
	}

	return scan.Err()
}

func (p *Porcelain) parseHeader(line string) error {
	const (
		initialPrefix = "## No commits yet on "
		detachedStr   = "## HEAD (no branch)"
	)

	switch {
	case line == detachedStr:
		p.IsDetached = true
	case strings.HasPrefix(line, initialPrefix):
		p.IsInitial = true
		p.LocalBranch = line[len(initialPrefix):]
	default:
		// regular branch[...remote] output, with or without ahead/behind counts
		if len(line) < 4 {
			// branch name is at least one character
			return errUnexpectedHeader
		}
		// check if a remote tracking branch is specified
		pos := strings.Index(line, "...")
		if pos == -1 {
			// we should have the branch name and nothing else, where spaces
			// are not allowed
			if strings.IndexByte(line[3:], ' ') != -1 {
				return errUnexpectedHeader
			}
			p.LocalBranch = line[3:]
		} else {
			p.LocalBranch = line[3:pos]
			p.parseUpstream(line[pos+3:])
		}
	}

	return nil
}

// parseUpstream parses the remote branch name and if present, its divergence
// with local branch (ahead / behind count)
func (p *Porcelain) parseUpstream(s string) error {
	var err error

	pos := strings.IndexByte(s, ' ')
	if pos == -1 {
		p.RemoteBranch = s
		return nil
	}
	p.RemoteBranch = s[:pos]
	s = strings.Trim(s[pos+1:], "[]")

	hasAhead := strings.Contains(s, "ahead")
	hasBehind := strings.Contains(s, "behind")

	switch {
	case hasAhead && hasBehind:
		_, err = fmt.Sscanf(s, "ahead %d, behind %d", &p.AheadCount, &p.BehindCount)
	case hasAhead:
		_, err = fmt.Sscanf(s, "ahead %d", &p.AheadCount)
	case hasBehind:
		_, err = fmt.Sscanf(s, "behind %d", &p.BehindCount)
	default:
		err = fmt.Errorf(`unexpected string "%s"`, s)
	}

	if err != nil {
		return fmt.Errorf("%v: %w", errParseAheadBehind, err)
	}
	return nil
}

type linecount int

// parseFrom counts the number of lines by reading from r.
func (lc *linecount) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)

	for scan.Scan() {
		*lc++
	}

	return scan.Err()
}

type lines []string

// parseFrom appends to itself the lines it finds by reading r.
func (l *lines) parseFrom(r io.Reader) error {
	scan := bufio.NewScanner(r)
	scan.Split(bufio.ScanLines)

	for scan.Scan() {
		*l = append(*l, scan.Text())
	}

	return scan.Err()
}

var shortStatRx = regexp.MustCompile(`.* (\d+) insertion.* (\d+) deletion.*`)

type stats struct {
	insertions int
	deletions  int
}

// parseStatus parses porcelain status and fills it with r.
func (s *stats) parseFrom(r io.Reader) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	s.insertions, s.deletions, err = extractShortStat(b)
	return err
}

func extractShortStat(out []byte) (insertions, deletions int, err error) {
	splits := bytes.Split(out, []byte{','})
	for j := range splits {
		line := bytes.TrimSpace(splits[j])
		if pos := bytes.Index(line, []byte("insertion")); pos != -1 {
			insertions, err = strconv.Atoi(string(bytes.TrimSpace(line[:pos])))
			if err != nil {
				return
			}
		} else if pos := bytes.Index(line, []byte("deletion")); pos != -1 {
			deletions, err = strconv.Atoi(string(bytes.TrimSpace(line[:pos])))
			if err != nil {
				return
			}
		}
	}

	return
}
