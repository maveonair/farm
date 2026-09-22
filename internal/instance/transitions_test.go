package instance

import "testing"

func TestTransition(t *testing.T) {
	tests := []struct {
		from State
		to   State
		want bool
	}{
		{StateBootstrapping, StateReady, true},
		{StateBootstrapping, StateRunning, true},
		{StateBootstrapping, StateCleaning, true},
		{StateReady, StateRunning, true},
		{StateRunning, StateReady, false},
		{StateReady, StateCleaning, true},
		{StateRunning, StateCleaning, true},
		{StateCleaning, StateFinished, true},
		{StateFinished, StateReady, false},
		{StateReady, StateBootstrapping, false},
	}

	for _, test := range tests {
		t.Run(string(test.from)+"_to_"+string(test.to), func(t *testing.T) {
			if got := CanTransition(test.from, test.to); got != test.want {
				t.Fatalf("CanTransition() = %t, want %t", got, test.want)
			}
		})
	}
}
