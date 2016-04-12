package stat

type ContainerStats struct {
        ID               string
        CPUPercentage    float64
        Memory           float64
        MemoryLimit      float64
        MemoryPercentage float64
        NetworkRx        float64
        NetworkTx        float64
        BlockRead        float64
        BlockWrite       float64
        PidsCurrent      uint64
        Name             string
}
