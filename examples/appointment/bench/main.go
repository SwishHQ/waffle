// Command bench is a waffle load test shaped like a Gotenberg docx->PDF
// conversion benchmark, so an in-process JSX->PDF render can be compared against
// a LibreOffice-in-a-sidecar pipeline apples-to-apples. It renders a multi-page
// offer/appointment letter with a full-page letterhead, embedded fonts, inline
// runs and a compensation table, and reports the same numbers: latency
// distribution, throughput, failures, and peak RSS.
//
// The template is compiled once (like Gotenberg keeping LibreOffice warm) and
// then rendered REQUESTS times at CONCURRENCY. Run it inside a container capped
// to the same profile Gotenberg is typically sized under (0.25 / 0.5 vCPU,
// capped memory, swap disabled) via bench.sh, which captures peak memory.
//
//	REQUESTS=30 CONCURRENCY=1 ./waffle-bench      # reads ./appointment.jsx + ./assets
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/swish/waffle"
)

func main() {
	jsxPath := getenv("WAFFLE_JSX", "appointment.jsx")
	src, err := os.ReadFile(jsxPath)
	if err != nil {
		fatalf("read %s: %v", jsxPath, err)
	}

	// Compile once — the warm path, mirroring --libreoffice-restart-after=10.
	compileStart := time.Now()
	tmpl, err := waffle.LoadTemplate(src, waffle.TemplateOptions{Filename: jsxPath})
	if err != nil {
		fatalf("compile template: %v", err)
	}
	compileDur := time.Since(compileStart)
	props := sampleProps()

	// One render to measure the output size and warm decode/JIT paths.
	var sz countWriter
	if _, err := tmpl.Render(context.Background(), props, &sz); err != nil {
		fatalf("warmup render: %v", err)
	}
	if _, err := tmpl.Render(context.Background(), props, io.Discard); err != nil {
		fatalf("warmup render: %v", err)
	}

	requests := envInt("REQUESTS", 30)
	concurrency := envInt("CONCURRENCY", 1)
	if concurrency > requests {
		concurrency = requests
	}

	// CPUPROFILE=/path/cpu.prof profiles the steady-state render loop only
	// (compile + warmup excluded), for `go tool pprof`.
	if pp := os.Getenv("CPUPROFILE"); pp != "" {
		f, err := os.Create(pp)
		if err != nil {
			fatalf("create profile: %v", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fatalf("start profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	type result struct {
		d   time.Duration
		err error
	}
	jobs := make(chan int, requests)
	out := make(chan result, requests)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				start := time.Now()
				_, err := tmpl.Render(context.Background(), props, io.Discard)
				out <- result{d: time.Since(start), err: err}
			}
		}()
	}

	wallStart := time.Now()
	for i := 0; i < requests; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	close(out)
	wall := time.Since(wallStart)

	var lat []time.Duration
	var ok, fail int
	var firstErr error
	for r := range out {
		if r.err != nil {
			fail++
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		ok++
		lat = append(lat, r.d)
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	fmt.Printf("engine: waffle (in-process goja + esbuild; no sidecar, no cgo)\n")
	fmt.Printf("document: %s -> PDF %d KB\n", jsxPath, sz.n/1024)
	fmt.Printf("one-time compile (LoadTemplate): %s\n", rnd(compileDur))
	fmt.Printf("GOMAXPROCS=%d  numCPU=%d\n", runtime.GOMAXPROCS(0), runtime.NumCPU())
	fmt.Printf("requests=%d concurrency=%d  ok=%d fail=%d  wall=%s\n",
		requests, concurrency, ok, fail, wall.Round(time.Millisecond))
	if ok > 0 {
		fmt.Printf("latency  min=%s  p50=%s  p95=%s  max=%s  mean=%s\n",
			rnd(lat[0]), rnd(pct(lat, 50)), rnd(pct(lat, 95)), rnd(lat[len(lat)-1]), rnd(mean(lat)))
		fmt.Printf("throughput: %.2f render/s\n", float64(ok)/wall.Seconds())
	}
	fmt.Printf("go mem: heapInUse=%d MiB  sys=%d MiB  numGC=%d\n",
		ms.HeapInuse>>20, ms.Sys>>20, ms.NumGC)
	if hwm := procPeakRSSKiB(); hwm > 0 {
		fmt.Printf("process peak RSS (VmHWM): %d MiB\n", hwm/1024)
	}
	if peak := cgroupPeakBytes(); peak > 0 {
		fmt.Printf("cgroup peak memory: %d MiB\n", peak>>20)
	}
	if fail > 0 {
		fatalf("%d/%d renders FAILED — first error: %v", fail, requests, firstErr)
	}
}

// sampleProps mirrors examples/appointment/main.go: a filled offer/appointment
// letter with a full compensation breakdown.
func sampleProps() map[string]any {
	return map[string]any{
		"date":        "15-05-2026",
		"candidate":   "Ravi Kumar",
		"father":      "Suresh Kumar",
		"addr1":       "No. 12, 4th Cross, MG Road",
		"addr2":       "Indiranagar, Bengaluru",
		"pincode":     "560038",
		"name":        "Ravi",
		"designation": "Operations Associate",
		"joining":     "01-06-2026",
		"location":    "Bengaluru",
		"salary":      "3,60,000",
		"location2":   "Bengaluru",
		"employee":    "Ravi Kumar",
		"acceptDate":  "01-06-2026",
		"comp": map[string]any{
			"ctc":      map[string]string{"m": "30,000", "a": "3,60,000"},
			"basic":    map[string]string{"m": "15,000", "a": "1,80,000"},
			"hra":      map[string]string{"m": "6,000", "a": "72,000"},
			"special":  map[string]string{"m": "7,200", "a": "86,400"},
			"gross":    map[string]string{"m": "28,200", "a": "3,38,400"},
			"pfEr":     map[string]string{"m": "1,800", "a": "21,600"},
			"pfEp":     map[string]string{"m": "1,800", "a": "21,600"},
			"pt":       map[string]string{"m": "200", "a": "2,400"},
			"esic":     map[string]string{"m": "0", "a": "0"},
			"totalDed": map[string]string{"m": "2,000", "a": "24,000"},
			"net":      map[string]string{"m": "26,200", "a": "3,14,400"},
		},
	}
}

type countWriter struct{ n int }

func (c *countWriter) Write(p []byte) (int, error) { c.n += len(p); return len(p), nil }

// procPeakRSSKiB reads VmHWM (peak resident set size) from /proc/self/status,
// in KiB. Returns 0 off Linux.
func procPeakRSSKiB() int {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				if n, err := strconv.Atoi(f[1]); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// cgroupPeakBytes reads the cgroup-v2 memory.peak, the container's peak memory —
// the same figure a Gotenberg sizing harness captures. Returns 0 if unavailable.
func cgroupPeakBytes() int64 {
	b, err := os.ReadFile("/sys/fs/cgroup/memory.peak")
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

func rnd(d time.Duration) time.Duration { return d.Round(time.Millisecond) }

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "waffle-bench: "+format+"\n", a...)
	os.Exit(1)
}
