package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"
)

type ReadConfig struct {
	Size     uint64 `json:"size"`     // testing dataset size(pre-constructed)
	KeySize  uint64 `json:"keysize"`  // size of each testing key
	DataSize uint64 `json:"datasize"` // size of each testing value

	LogPercent bool   `json:"-"`
	TestName   string `json:"-"`
}

type ReadEnv struct {
	cfg ReadConfig

	// generating keys and values
	key, value []byte
	rand       *rand.Rand
	log        *json.Encoder
	kw         io.Writer
	kr         io.Reader
	resetKey   func()
	keych      chan [][]byte

	// reporting
	mu                  sync.Mutex
	startTime, lastTime time.Duration
	read, lastRead      int
	lastPercent         int
}

func NewReadEnv(log io.Writer, kr io.Reader, kw io.Writer, resetKey func(), cfg ReadConfig) *ReadEnv {
	return &ReadEnv{
		cfg:      cfg,
		log:      json.NewEncoder(log),
		kr:       kr,
		kw:       kw,
		resetKey: resetKey,
		key:      make([]byte, cfg.KeySize),
		value:    make([]byte, cfg.DataSize),
		keych:    make(chan [][]byte, 100),
	}
}

// Run calls write repeatedly with random keys and values.
// The write function should perform a database write and call LegacyWriteProgress when
// data has actually been flushed to disk.
func (env *ReadEnv) Run(write func(key, value string, lastCall bool) error, read func(key string) error) error {
	env.start()

	var (
		err      error
		written  = uint64(0)
		keypool  [][]byte
		wg       sync.WaitGroup
		result   = make(chan [][]byte, 100)
		shutdown = make(chan struct{})
	)
	defer func() {
		close(shutdown)
		wg.Wait()
	}()

	// Stage one, construct the test dataset
	if env.kw != nil {
		wg.Add(1)
		go env.writeKey(shutdown, &wg)
	stageOne:
		for {
			env.rand.Read(env.key)
			env.rand.Read(env.value)

			written += env.cfg.DataSize
			end := written >= env.cfg.Size
			err = write(string(env.key), string(env.value), end)
			if err != nil || end {
				if err == nil {
					keypool = append(keypool, copyBytes(env.key))
				}
				if len(keypool) > 0 {
					env.keych <- keypool
					keypool = make([][]byte, 0)
				}
				break stageOne
			}
			keypool = append(keypool, copyBytes(env.key))
			if len(keypool) > 1024 {
				env.keych <- keypool
				keypool = make([][]byte, 0)
			}
		}
		if err != nil {
			return err
		}
	}

	// Stage two, read bench
	wg.Add(1)
	go env.readKey(result, shutdown, &wg)

stageTwo:
	for keybatch := range result {
		for _, key := range keybatch {
			err = read(string(key))
			if err != nil {
				break stageTwo
			}
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (env *ReadEnv) writeKey(shutdown chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case batchKeys := <-env.keych:
			var buffer []byte
			for _, key := range batchKeys {
				buffer = append(buffer, key...)
			}
			env.kw.Write(buffer)
		case <-shutdown:
			return
		}
	}
}

func (env *ReadEnv) readKey(result chan [][]byte, shutdown chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	var buffer = make([]byte, env.cfg.KeySize*1024)
	if env.resetKey != nil {
		env.resetKey()
	}
	for {
		read, err := env.kr.Read(buffer)
		if read == 0 {
			close(result)
			return
		}
		var batchKey = make([][]byte, read/int(env.cfg.KeySize))
		for i := 0; i+int(env.cfg.KeySize) <= read; i += int(env.cfg.KeySize) {
			batchKey[i/int(env.cfg.KeySize)] = copyBytes(buffer[i : i+int(env.cfg.KeySize)])
		}
		select {
		case result <- batchKey:
		case <-shutdown:
			return
		}
		if err != nil {
			close(result)
			return
		}
	}
}

func (env *ReadEnv) start() {
	env.rand = rand.New(rand.NewSource(0x1334))
	env.startTime = mononow()
	env.lastTime = env.startTime
}

// Progress writes a JSON progress event to the environment's output writer.
func (env *ReadEnv) Progress(w int) {
	now := mononow()
	env.mu.Lock()
	defer env.mu.Unlock()
	env.read += w
	d := now - env.lastTime
	dw := env.read - env.lastRead
	if dw > 0 && dw > emitInterval {
		p := Progress{Processed: env.read, Delta: dw, Duration: d}
		env.log.Encode(&p)
		env.logPercentage()
		env.lastTime = now
		env.lastRead = env.read
	}
}

func (env *ReadEnv) logPercentage() {
	if !env.cfg.LogPercent {
		return
	}
	pct := int((float64(env.read) / float64(env.cfg.Size)) * 100)
	if pct > env.lastPercent {
		fmt.Printf("%3d%%  %s\n", pct, env.cfg.TestName)
		env.lastPercent = pct
	}
}
