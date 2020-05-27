package bench

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"time"
)

const emitInterval = 500 * 1024 // bytes

type WriteConfig struct {
	Size     uint64 `json:"size"`     // total size of values to write
	KeySize  uint64 `json:"keysize"`  // size of each key written
	DataSize uint64 `json:"datasize"` // size of each value written

	LogPercent bool   `json:"-"`
	TestName   string `json:"-"`
}

type WriteEnv struct {
	cfg WriteConfig
	// generating keys and values
	key, value []byte
	rand       *rand.Rand
	out        *json.Encoder
	// reporting
	mu                   sync.Mutex
	startTime, lastTime  time.Duration
	written, lastWritten uint64
	lastPercent          int
}

func NewWriteEnv(output io.Writer, cfg WriteConfig) *WriteEnv {
	return &WriteEnv{
		cfg:   cfg,
		out:   json.NewEncoder(output),
		key:   make([]byte, cfg.KeySize),
		value: make([]byte, cfg.DataSize),
	}
}

// Run calls write repeatedly with random keys and values.
// The write function should perform a database write and call LegacyWriteProgress when
// data has actually been flushed to disk.
func (env *WriteEnv) Run(write func(key, value string, lastCall bool) error) error {
	env.start()
	written := uint64(0)
	for {
		env.rand.Read(env.key)
		env.rand.Read(env.value)
		written += env.cfg.DataSize
		end := written >= env.cfg.Size
		err := write(string(env.key), string(env.value), end)
		if err != nil || end {
			return err
		}
	}
}

func (env *WriteEnv) start() {
	env.written, env.lastWritten = 0, 0
	env.rand = rand.New(rand.NewSource(0x1334))
	env.startTime = mononow()
	env.lastTime = env.startTime
}

// LegacyWriteProgress writes a JSON progress event to the environment's output writer.
func (env *WriteEnv) Progress(w int) {
	now := mononow()
	env.mu.Lock()
	defer env.mu.Unlock()
	env.written += uint64(w)
	d := now - env.lastTime
	dw := env.written - env.lastWritten
	if dw > 0 && dw > emitInterval {
		p := Progress{Processed: env.written, Delta: dw, Duration: d}
		env.out.Encode(&p)
		env.logPercentage()
		env.lastTime = now
		env.lastWritten = env.written
	}
}

func (env *WriteEnv) logPercentage() {
	if !env.cfg.LogPercent {
		return
	}
	pct := int((float64(env.written) / float64(env.cfg.Size)) * 100)
	if pct > env.lastPercent {
		fmt.Printf("%3d%%  %s\n", pct, env.cfg.TestName)
		env.lastPercent = pct
	}
}
