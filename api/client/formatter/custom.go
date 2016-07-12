package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/client/stat"
	"github.com/docker/docker/pkg/stringid"
	"github.com/docker/docker/pkg/stringutils"
	"github.com/docker/engine-api/types"
	"github.com/docker/go-units"
)

const (
	tableKey = "table"

	containerIDHeader  = "CONTAINER ID"
	imageHeader        = "IMAGE"
	namesHeader        = "NAMES"
	commandHeader      = "COMMAND"
	createdSinceHeader = "CREATED"
	createdAtHeader    = "CREATED AT"
	runningForHeader   = "CREATED"
	statusHeader       = "STATUS"
	portsHeader        = "PORTS"
	sizeHeader         = "SIZE"
	labelsHeader       = "LABELS"
	imageIDHeader      = "IMAGE ID"
	repositoryHeader   = "REPOSITORY"
	tagHeader          = "TAG"
	digestHeader       = "DIGEST"
	mountsHeader       = "MOUNTS"
	containerHeader    = "CONTAINER"
	nameHeader         = "CONTAINER NAMES"
	statsCPUHeader     = "CPU %"
	statsMemUsageHeader= "MEM USAGE / LIMIT"
	statsMemHeader     = "MEM %"
	statsNetHeader     = "NET I/O"
	statsBlockHeader   = "BLOCK I/O"
	statsPidHeader     = "PIDS"
)

type containerContext struct {
	baseSubContext
	trunc bool
	c     types.Container
}

func (c *containerContext) ID() string {
	c.addHeader(containerIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.c.ID)
	}
	return c.c.ID
}

func (c *containerContext) Names() string {
	c.addHeader(namesHeader)
	names := stripNamePrefix(c.c.Names)
	if c.trunc {
		for _, name := range names {
			if len(strings.Split(name, "/")) == 1 {
				names = []string{name}
				break
			}
		}
	}
	return strings.Join(names, ",")
}

func (c *containerContext) Image() string {
	c.addHeader(imageHeader)
	if c.c.Image == "" {
		return "<no image>"
	}
	if c.trunc {
		if trunc := stringid.TruncateID(c.c.ImageID); trunc == stringid.TruncateID(c.c.Image) {
			return trunc
		}
	}
	return c.c.Image
}

func (c *containerContext) Command() string {
	c.addHeader(commandHeader)
	command := c.c.Command
	if c.trunc {
		command = stringutils.Truncate(command, 20)
	}
	return strconv.Quote(command)
}

func (c *containerContext) CreatedAt() string {
	c.addHeader(createdAtHeader)
	return time.Unix(int64(c.c.Created), 0).String()
}

func (c *containerContext) RunningFor() string {
	c.addHeader(runningForHeader)
	createdAt := time.Unix(int64(c.c.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *containerContext) Ports() string {
	c.addHeader(portsHeader)
	return api.DisplayablePorts(c.c.Ports)
}

func (c *containerContext) Status() string {
	c.addHeader(statusHeader)
	return c.c.Status
}

func (c *containerContext) Size() string {
	c.addHeader(sizeHeader)
	srw := units.HumanSize(float64(c.c.SizeRw))
	sv := units.HumanSize(float64(c.c.SizeRootFs))

	sf := srw
	if c.c.SizeRootFs > 0 {
		sf = fmt.Sprintf("%s (virtual %s)", srw, sv)
	}
	return sf
}

func (c *containerContext) Labels() string {
	c.addHeader(labelsHeader)
	if c.c.Labels == nil {
		return ""
	}

	var joinLabels []string
	for k, v := range c.c.Labels {
		joinLabels = append(joinLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(joinLabels, ",")
}

func (c *containerContext) Label(name string) string {
	n := strings.Split(name, ".")
	r := strings.NewReplacer("-", " ", "_", " ")
	h := r.Replace(n[len(n)-1])

	c.addHeader(h)

	if c.c.Labels == nil {
		return ""
	}
	return c.c.Labels[name]
}

func (c *containerContext) Mounts() string {
	c.addHeader(mountsHeader)

	var name string
	var mounts []string
	for _, m := range c.c.Mounts {
		if m.Name == "" {
			name = m.Source
		} else {
			name = m.Name
		}
		if c.trunc {
			name = stringutils.Truncate(name, 15)
		}
		mounts = append(mounts, name)
	}
	return strings.Join(mounts, ",")
}

type imageContext struct {
	baseSubContext
	trunc  bool
	i      types.Image
	repo   string
	tag    string
	digest string
}

func (c *imageContext) ID() string {
	c.addHeader(imageIDHeader)
	if c.trunc {
		return stringid.TruncateID(c.i.ID)
	}
	return c.i.ID
}

func (c *imageContext) Repository() string {
	c.addHeader(repositoryHeader)
	return c.repo
}

func (c *imageContext) Tag() string {
	c.addHeader(tagHeader)
	return c.tag
}

func (c *imageContext) Digest() string {
	c.addHeader(digestHeader)
	return c.digest
}

func (c *imageContext) CreatedSince() string {
	c.addHeader(createdSinceHeader)
	createdAt := time.Unix(int64(c.i.Created), 0)
	return units.HumanDuration(time.Now().UTC().Sub(createdAt))
}

func (c *imageContext) CreatedAt() string {
	c.addHeader(createdAtHeader)
	return time.Unix(int64(c.i.Created), 0).String()
}

func (c *imageContext) Size() string {
	c.addHeader(sizeHeader)
	return units.HumanSize(float64(c.i.Size))
}

type subContext interface {
	fullHeader() string
	addHeader(header string)
}

type baseSubContext struct {
	header []string
}

func (c *baseSubContext) fullHeader() string {
	if c.header == nil {
		return ""
	}
	return strings.Join(c.header, "\t")
}

func (c *baseSubContext) addHeader(header string) {
	if c.header == nil {
		c.header = []string{}
	}
	c.header = append(c.header, strings.ToUpper(header))
}

func stripNamePrefix(ss []string) []string {
	sss := make([]string, len(ss))
	for i, s := range ss {
		sss[i] = s[1:]
	}

	return sss
}

type containerStatsContext struct {
	baseSubContext
	trunc bool
	s     stat.ContainerStats
}

func (c *containerStatsContext) ID() string {
	c.addHeader(containerHeader)
	if c.trunc {
		return stringid.TruncateID(c.s.ID)
	}
	return c.s.ID
}

func (c *containerStatsContext) Name() string {
	c.addHeader(nameHeader)
	return c.s.Name
}

func (c *containerStatsContext) Cpu() string {
	c.addHeader(statsCPUHeader)
	perc := c.s.CPUPercentage
	percentage := fmt.Sprintf("%.2f%%", perc)
	return percentage
}

func (c *containerStatsContext) MemUsage() string {
	c.addHeader(statsMemUsageHeader)
	usage := units.BytesSize(c.s.Memory)
	limit := units.BytesSize(float64(c.s.MemoryLimit))

	mu := fmt.Sprintf("%s / %s", usage, limit)
	return mu
}

func (c *containerStatsContext) Mem() string {
	c.addHeader(statsMemHeader)
	perc := c.s.MemoryPercentage
	percentage := fmt.Sprintf("%.2f%%", perc)
	return percentage
}

func (c *containerStatsContext) NetIO() string {
	c.addHeader(statsNetHeader)
	NetRx := units.HumanSize(c.s.NetworkRx)
	NetTx := units.HumanSize(float64(c.s.NetworkTx))

	net := fmt.Sprintf("%s / %s", NetRx, NetTx)
	return net
}

func (c *containerStatsContext) BlockIO() string {
	c.addHeader(statsBlockHeader)
	read := units.HumanSize(c.s.BlockRead)
	write := units.HumanSize(float64(c.s.BlockWrite))

	block := fmt.Sprintf("%s / %s", read, write)
	return block
}

func (c *containerStatsContext) Pid() string {
	c.addHeader(statsPidHeader)
	pid := fmt.Sprintf("%d", c.s.PidsCurrent)
	return pid
}
