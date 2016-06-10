package client

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/client/formatter"
	"github.com/docker/docker/api/client/stat"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

type containerStats struct {
	cs    stat.ContainerStats
	mu    sync.RWMutex
	err   error
}

type stats struct {
	mu sync.Mutex
	cs []*containerStats
}

func (s *stats) add(cs *containerStats) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.isKnownContainer(cs.cs.ID); !exists {
		s.cs = append(s.cs, cs)
		return true
	}
	return false
}

func (s *stats) remove(id string) {
	s.mu.Lock()
	if i, exists := s.isKnownContainer(id); exists {
		s.cs = append(s.cs[:i], s.cs[i+1:]...)
	}
	s.mu.Unlock()
}

func (s *stats) isKnownContainer(cid string) (int, bool) {
	for i, c := range s.cs {
		if c.cs.ID == cid {
			return i, true
		}
	}
	return -1, false
}

func (s *containerStats) Collect(ctx context.Context, cli client.APIClient, streamStats bool, waitFirst *sync.WaitGroup) {
	var (
		getFirst       bool
		previousCPU    uint64
		previousSystem uint64
		u              = make(chan error, 1)
	)

	defer func() {
		// if error happens and we get nothing of stats, release wait group whatever
		if !getFirst {
			getFirst = true
			waitFirst.Done()
		}
	}()

	responseBody, err := cli.ContainerStats(ctx, s.cs.Name, streamStats)
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return
	}
	defer responseBody.Close()

	dec := json.NewDecoder(responseBody)
	go func() {
		for {
			var v *types.StatsJSON

			if err := dec.Decode(&v); err != nil {
				dec = json.NewDecoder(io.MultiReader(dec.Buffered(), responseBody))
				u <- err
				continue
			}

			var memPercent = 0.0
			var cpuPercent = 0.0

			// MemoryStats.Limit will never be 0 unless the container is not running and we haven't
			// got any data from cgroup
			if v.MemoryStats.Limit != 0 {
				memPercent = float64(v.MemoryStats.Usage) / float64(v.MemoryStats.Limit) * 100.0
			}

			previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
			previousSystem = v.PreCPUStats.SystemUsage
			cpuPercent = calculateCPUPercent(previousCPU, previousSystem, v)
			blkRead, blkWrite := calculateBlockIO(v.BlkioStats)
			s.mu.Lock()
			s.cs.CPUPercentage = cpuPercent
			s.cs.Memory = float64(v.MemoryStats.Usage)
			s.cs.MemoryLimit = float64(v.MemoryStats.Limit)
			s.cs.MemoryPercentage = memPercent
			s.cs.NetworkRx, s.cs.NetworkTx = calculateNetwork(v.Networks)
			s.cs.BlockRead = float64(blkRead)
			s.cs.BlockWrite = float64(blkWrite)
			s.cs.PidsCurrent = v.PidsStats.Current
			s.mu.Unlock()
			u <- nil
			if !streamStats {
				return
			}
		}
	}()
	for {
		select {
		case <-time.After(2 * time.Second):
			// zero out the values if we have not received an update within
			// the specified duration.
			s.mu.Lock()
			s.cs.CPUPercentage = 0
			s.cs.Memory = 0
			s.cs.MemoryPercentage = 0
			s.cs.MemoryLimit = 0
			s.cs.NetworkRx = 0
			s.cs.NetworkTx = 0
			s.cs.BlockRead = 0
			s.cs.BlockWrite = 0
			s.cs.PidsCurrent = 0
			s.mu.Unlock()
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		case err := <-u:
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				continue
			}
			s.err = nil
			// if this is the first stat you get, release WaitGroup
			if !getFirst {
				getFirst = true
				waitFirst.Done()
			}
		}
		if !streamStats {
			return
		}
	}
}

func Display(cli *DockerCli, format *string, trunc bool, cs []*containerStats) {
	//s.mu.RLock()
	//defer s.mu.RUnlock()
	//if s.err != nil {
		//format = "%s\t%s\t%s / %s\t%s\t%s / %s\t%s / %s\t%s\n"
		//errStr := "--"
		//fmt.Fprintf(w, format,
			//s.Name, errStr, errStr, errStr, errStr, errStr, errStr, errStr, errStr, errStr,
		//)
		//err := s.err
		//return err
	//}
	stats := []stat.ContainerStats{}
	for _, s := range cs {
		s.mu.RLock()
		stats = append(stats, s.cs)
		s.mu.RUnlock()
	}
	f := *format
	if len(f) == 0 {
		if len(cli.StatsFormat()) > 0{
			f = cli.StatsFormat()
		} else {
			f = "table"
		}
	}
	
	statsCtx := formatter.ContainerStatsContext{
		Context: formatter.Context{
			Output: cli.out,
			Format: f,
			Quiet:  false,
			Trunc:  trunc,
		},
		ShowName: strings.ToLower(f) == "names",
		Stats:    stats,
		//Stats: []types.ContainerStats{s.Name, s.CPUPercentage, s.Memory, s.MemoryLimit, s.MemoryPercentage, s.NetworkRx, s.NetworkTx, s.BlockRead, s.BlockWrite, s.PidsCurrent},
	}

	statsCtx.Write()
//	return nil
}

func calculateCPUPercent(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return cpuPercent
}

func calculateBlockIO(blkio types.BlkioStats) (blkRead uint64, blkWrite uint64) {
	for _, bioEntry := range blkio.IoServiceBytesRecursive {
		switch strings.ToLower(bioEntry.Op) {
		case "read":
			blkRead = blkRead + bioEntry.Value
		case "write":
			blkWrite = blkWrite + bioEntry.Value
		}
	}
	return
}

func calculateNetwork(network map[string]types.NetworkStats) (float64, float64) {
	var rx, tx float64

	for _, v := range network {
		rx += float64(v.RxBytes)
		tx += float64(v.TxBytes)
	}
	return rx, tx
}
