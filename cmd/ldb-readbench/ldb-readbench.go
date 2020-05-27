package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bench "github.com/fjl/goleveldb-bench"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func main() {
	var (
		testflag     = flag.String("test", "", "tests to run ("+strings.Join(testnames(), ", ")+")")
		sizeflag     = flag.String("size", "500mb", "total amount of value data to write")
		datasizeflag = flag.String("valuesize", "100b", "size of each value")
		keysizeflag  = flag.String("keysize", "32b", "size of each key")
		dirflag      = flag.String("dir", ".", "test database directory")
		logdirflag   = flag.String("logdir", ".", "test log output directory")
		deletedbflag = flag.Bool("deletedb", false, "delete databases after test run")

		run []string
		cfg bench.ReadConfig
		err error
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
	if cfg.Size, err = bench.ParseSize(*sizeflag); err != nil {
		log.Fatal("-size: ", err)
	}
	if cfg.DataSize, err = bench.ParseSize(*datasizeflag); err != nil {
		log.Fatal("-datasize: ", err)
	}
	if cfg.KeySize, err = bench.ParseSize(*keysizeflag); err != nil {
		log.Fatal("-datasize: ", err)
	}
	cfg.LogPercent = true

	if err := os.MkdirAll(*logdirflag, 0755); err != nil {
		log.Fatal("can't create log dir: %v", err)
	}

	anyErr := false
	for _, name := range run {
		var dbdir string
		if strings.Contains(*dirflag, "testdb-") {
			if hasFilter(*dirflag) != hasFilter(name) {
				log.Printf("Skip test %s. Incompatible database", name)
				continue
			}
			dbdir = *dirflag // The dirflag points to an existent database, reuse it.
		} else {
			dbdir = filepath.Join(*dirflag, "testdb-"+name)
		}
		if err := os.MkdirAll(dbdir, 0755); err != nil {
			log.Fatal("can't create log dir: %v", err)
		}
		if err := runTest(*logdirflag, dbdir, name, cfg); err != nil {
			log.Printf("test %q failed: %v", name, err)
			anyErr = true
		}
		if *deletedbflag {
			os.RemoveAll(dbdir)
		}
	}
	if anyErr {
		log.Fatal("one ore more tests failed")
	}
}

func runTest(logdir, dbdir, name string, cfg bench.ReadConfig) error {
	cfg.TestName = name
	logfile, err := os.Create(filepath.Join(logdir, name+time.Now().Format(".2006-01-02-15:04:05")+".json"))
	if err != nil {
		return err
	}
	defer logfile.Close()

	var (
		kw    io.Writer
		kr    io.Reader
		reset func()
	)
	kfile := filepath.Join(dbdir, "testing.key")
	if fileExist(kfile) {
		keyfile, err := os.Open(kfile)
		if err != nil {
			return err
		}
		defer keyfile.Close()
		kr = keyfile
	} else {
		keyfile, err := os.Create(kfile)
		if err != nil {
			return err
		}
		defer keyfile.Close()
		kw, kr = keyfile, keyfile
		reset = func() {
			keyfile.Seek(0, io.SeekStart)
		}
	}

	log.Printf("== running %q", name)
	env := bench.NewReadEnv(logfile, kr, kw, reset, cfg)
	return tests[name].Benchmark(dbdir, env)
}

type Benchmarker interface {
	Benchmark(dir string, env *bench.ReadEnv) error
}

var tests = map[string]Benchmarker{
	"random-read": randomRead{},
	"random-read-filter": randomRead{Options: opt.Options{
		Filter: filter.NewBloomFilter(10),
	}},
	"random-read-bigcache": randomRead{Options: opt.Options{
		BlockCacheCapacity: 100 * opt.MiB,
	}},
	"random-read-bigcache-filter": randomRead{Options: opt.Options{
		BlockCacheCapacity: 100 * opt.MiB,
		Filter:             filter.NewBloomFilter(10),
	}},
	"random-read-metacache-filter": randomRead{Options: opt.Options{
		MetadataCacheCapacity: 100 * opt.MiB,
		Filter:                filter.NewBloomFilter(10),
	}},
}

func testnames() (n []string) {
	for name := range tests {
		n = append(n, name)
	}
	sort.Strings(n)
	return n
}

type randomRead struct {
	Options opt.Options
}

func (b randomRead) Benchmark(dir string, env *bench.ReadEnv) error {
	db, err := leveldb.OpenFile(dir, &b.Options)
	if err != nil {
		return err
	}
	defer db.Close()
	return env.Run(func(key, value string, lastCall bool) error {
		if err := db.Put([]byte(key), []byte(value), nil); err != nil {
			return err
		}
		return nil
	}, func(key string) error {
		if value, err := db.Get([]byte(key), nil); err != nil {
			env.RecordMiss()
		} else {
			env.Progress(len(value))
		}
		return nil
	})
}

func fileExist(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func hasFilter(name string) bool {
	return strings.Contains(name, "filter")
}
