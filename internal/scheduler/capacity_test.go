package scheduler

import (
	"testing"

	"github.com/maveonair/farm/internal/instance"
)

func TestNeeded(t *testing.T) {
	tests := []struct {
		name     string
		capacity Capacity
		want     int
	}{
		{
			name: "waiting jobs",
			capacity: Capacity{
				Waiting:         3,
				MaxInstances:    5,
				MaxProvisioning: 5,
			},
			want: 3,
		},
		{
			name: "ready and provisioning capacity",
			capacity: Capacity{
				Waiting:         3,
				MaxInstances:    5,
				MaxProvisioning: 5,
				Instances: []instance.Instance{
					{State: instance.StateReady},
					{State: instance.StateBootstrapping},
				},
			},
			want: 1,
		},
		{
			name: "warm capacity",
			capacity: Capacity{
				MinIdle:         2,
				MaxInstances:    5,
				MaxProvisioning: 5,
			},
			want: 2,
		},
		{
			name: "instance limit",
			capacity: Capacity{
				Waiting:         4,
				MaxInstances:    2,
				MaxProvisioning: 4,
				Instances:       []instance.Instance{{State: instance.StateRunning}},
			},
			want: 1,
		},
		{
			name: "provisioning limit",
			capacity: Capacity{
				Waiting:         4,
				MaxInstances:    5,
				MaxProvisioning: 2,
				Instances:       []instance.Instance{{State: instance.StateBootstrapping}},
			},
			want: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Needed(test.capacity); got != test.want {
				t.Fatalf("Needed() = %d, want %d", got, test.want)
			}
		})
	}
}
