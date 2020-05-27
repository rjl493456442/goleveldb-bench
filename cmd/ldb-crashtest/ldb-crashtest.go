package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	// Run the test if we're the writer child process.
	if len(os.Args) > 1 && os.Args[1] == "-writer" {
		writer()
		return
	}

	// Be the front-end otherwise.
	var (
		testflag  = flag.String("test", "seq", "tests to run ("+strings.Join(testnames(), ", ")+")")
		timeflag  = flag.Duration("time", 30*time.Second, "time to wait before terminating the writer process")
		dirflag   = flag.String("dir", ".", "test database directory")
		countflag = flag.Uint("count", 1000, "number of test repetitions")
		run       []string
	)
	flag.Parse()

	for _, t := range strings.Split(*testflag, ",") {
		if tests[t] == nil {
			log.Fatalf("unknown test %q", t)
		}
		run = append(run, t)
	}
	if len(run) == 0 {
		log.Fatal("no tests to run, use -test to select tests")
	}

	anyErr := false
	for _, name := range run {
		for i := uint(1); i <= *countflag; i++ {
			log.Printf("== running test %q (%d/%d)", name, i, *countflag)
			if err := runTest(*dirflag, name, *timeflag); err != nil {
				log.Printf("test %q failed: %v", name, err)
				anyErr = true
			}
		}
	}
	if anyErr {
		log.Fatal("one ore more tests failed")
	}
}

func runTest(basedir, name string, avgwait time.Duration) error {
	thiscmd, err := os.Executable()
	if err != nil {
		log.Fatalf("can't figure out executable path: %v", err)
	}
	dbdir := filepath.Join(basedir, "testdb-crashtest-"+name)
	if err := os.RemoveAll(dbdir); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Start the writer process and terminate it on a randomized timeout.
	ctx, cancel := context.WithTimeout(context.Background(), randomWaitTime(avgwait))
	defer cancel()
	writer := exec.CommandContext(ctx, thiscmd, "-writer", dbdir, name)
	writer.Stdout, writer.Stderr = os.Stdout, os.Stderr
	writer.Run()

	return checkDB(dbdir)
}

func randomWaitTime(avg time.Duration) time.Duration {
	wiggle := 500 * time.Millisecond
	if wiggle > avg {
		wiggle = avg / 2
	}
	r := time.Duration(rand.Int63n(int64(wiggle)))
	return avg - wiggle/2 + r
}

// checkDB opens the database and checks whether all keys are present and
// the correct value is stored for each keys.
func checkDB(dbdir string) error {
	db, err := leveldb.OpenFile(dbdir, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	var checkErr error
	var maxIndex uint64
	iterateTestKeys(func(i uint64, k, v []byte) bool {
		value, err := db.Get(k, nil)
		if err != nil {
			return true
		}
		maxIndex = i
		if !bytes.Equal(value, v) {
			checkErr = fmt.Errorf("mismatch for key %x: want %x, found %x", k, value, v)
		}
		return checkErr != nil
	})
	log.Printf("  == database has keys up to %d", maxIndex)
	return checkErr
}

// iterateTestKeys calls fn with keys and values until it returns true.
// The keys and values are 32-byte values.
func iterateTestKeys(fn func(i uint64, k, v []byte) bool) {
	var n, k, v [32]byte
	hash := sha1.New()
	for i := uint64(0); ; i++ {
		binary.BigEndian.PutUint64(n[:], i)
		hash.Write(n[:])
		hash.Sum(k[:0])
		hash.Write(k[:])
		hash.Sum(v[:0])
		if fn(i, k[:], v[:]) {
			return
		}
		hash.Reset()
	}
}

// writer is the main function of the child process.
func writer() {
	if len(os.Args) != 4 {
		log.Fatal("invalid number of arguments")
	}
	dbdir, name := os.Args[2], os.Args[3]
	if err := tests[name].test(dbdir); err != nil {
		log.Fatal(err)
	}
}

// These are the different write modes.
var tests = map[string]tester{
	"seq":          seqWrite{sync: true},
	"seq-nosync":   seqWrite{sync: false},
	"batch":        batchWrite{sync: true, size: 10000},
	"batch-nosync": batchWrite{sync: false, size: 10000},
}

func testnames() (n []string) {
	for name := range tests {
		n = append(n, name)
	}
	sort.Strings(n)
	return n
}

type tester interface {
	test(dbdir string) error
}

type seqWrite struct {
	sync bool
}

func (t seqWrite) test(dbdir string) error {
	db, err := leveldb.OpenFile(dbdir, &opt.Options{NoSync: !t.sync})
	if err != nil {
		return err
	}
	iterateTestKeys(func(i uint64, k, v []byte) bool {
		db.Put(k, v, nil)
		if i > 0 && i%10000 == 0 {
			fmt.Printf("%d\n", i)
		}
		return false
	})
	return nil
}

type batchWrite struct {
	sync bool
	size int
}

func (t batchWrite) test(dbdir string) error {
	db, err := leveldb.OpenFile(dbdir, &opt.Options{NoSync: !t.sync})
	if err != nil {
		return err
	}
	var batch leveldb.Batch
	iterateTestKeys(func(i uint64, k, v []byte) bool {
		batch.Put(k, v)
		if i > 0 && i%10000 == 0 {
			db.Write(&batch, nil)
			batch.Reset()
			fmt.Printf("%d\n", i)
		}
		return false
	})
	return nil
}
