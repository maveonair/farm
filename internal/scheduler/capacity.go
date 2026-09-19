package scheduler

import "github.com/maveonair/farm/internal/instance"

type Capacity struct {
	Waiting         int
	MinIdle         int
	MaxInstances    int
	MaxProvisioning int
	Instances       []instance.Instance
}

type Demand struct {
	Waiting         int
	MinIdle         int
	MaxInstances    int
	MaxProvisioning int
	Ready           int
	Provisioning    int
	Instances       int
}

func Needed(capacity Capacity) int {
	var ready, provisioning int
	for _, record := range capacity.Instances {
		switch record.State {
		case instance.StateReady:
			ready++
		case instance.StateBootstrapping:
			provisioning++
		}
	}

	return NeededFor(Demand{
		Waiting: capacity.Waiting, MinIdle: capacity.MinIdle,
		MaxInstances: capacity.MaxInstances, MaxProvisioning: capacity.MaxProvisioning,
		Ready: ready, Provisioning: provisioning, Instances: len(capacity.Instances),
	})
}

func NeededFor(demand Demand) int {
	needed := demand.Waiting + demand.MinIdle - demand.Ready - demand.Provisioning
	if needed <= 0 {
		return 0
	}

	remainingInstances := demand.MaxInstances - demand.Instances
	if remainingInstances <= 0 {
		return 0
	}
	if needed > remainingInstances {
		needed = remainingInstances
	}

	remainingProvisioning := demand.MaxProvisioning - demand.Provisioning
	if remainingProvisioning <= 0 {
		return 0
	}
	if needed > remainingProvisioning {
		needed = remainingProvisioning
	}

	return needed
}
