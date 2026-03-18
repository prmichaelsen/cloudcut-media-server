package progress

import (
	"strings"
	"testing"
	"time"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		duration time.Duration
		want     *Progress
	}{
		{
			name:     "valid progress line",
			line:     "frame=  123 fps= 45 q=28.0 size=    1024kB time=00:00:05.12 bitrate=1638.4kbits/s speed=1.23x",
			duration: 10 * time.Second,
			want: &Progress{
				Frame:    123,
				FPS:      45.0,
				Bitrate:  "1638.4kbits/s",
				Time:     5*time.Second + 120*time.Millisecond,
				Speed:    1.23,
				Progress: 51.2, // 5.12s / 10s * 100
			},
		},
		{
			name:     "100 percent complete",
			line:     "frame=  300 fps= 60 q=28.0 size=    2048kB time=00:00:10.00 bitrate=1638.4kbits/s speed=2.00x",
			duration: 10 * time.Second,
			want: &Progress{
				Frame:    300,
				FPS:      60.0,
				Bitrate:  "1638.4kbits/s",
				Time:     10 * time.Second,
				Speed:    2.00,
				Progress: 100.0,
			},
		},
		{
			name:     "no progress data",
			line:     "Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'input.mp4':",
			duration: 10 * time.Second,
			want:     nil,
		},
		{
			name:     "partial progress line",
			line:     "frame=  50 fps= 30",
			duration: 10 * time.Second,
			want:     nil, // Incomplete line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.duration)
			got := parser.parseLine(tt.line)

			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}

			if got == nil {
				t.Errorf("expected %+v, got nil", tt.want)
				return
			}

			if got.Frame != tt.want.Frame {
				t.Errorf("Frame = %d, want %d", got.Frame, tt.want.Frame)
			}
			if got.FPS != tt.want.FPS {
				t.Errorf("FPS = %.1f, want %.1f", got.FPS, tt.want.FPS)
			}
			if got.Bitrate != tt.want.Bitrate {
				t.Errorf("Bitrate = %s, want %s", got.Bitrate, tt.want.Bitrate)
			}
			if got.Time != tt.want.Time {
				t.Errorf("Time = %v, want %v", got.Time, tt.want.Time)
			}
			if got.Speed != tt.want.Speed {
				t.Errorf("Speed = %.2f, want %.2f", got.Speed, tt.want.Speed)
			}

			// Allow small floating point differences
			if diff := got.Progress - tt.want.Progress; diff < -0.1 || diff > 0.1 {
				t.Errorf("Progress = %.2f, want %.2f", got.Progress, tt.want.Progress)
			}
		})
	}
}

func TestParse(t *testing.T) {
	input := `FFmpeg version info...
Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'input.mp4':
  Duration: 00:00:10.00, start: 0.000000, bitrate: 1000 kb/s
frame=   10 fps= 30 q=28.0 size=     256kB time=00:00:01.00 bitrate=2048.0kbits/s speed=1.00x
frame=   50 fps= 40 q=28.0 size=    1024kB time=00:00:05.00 bitrate=1638.4kbits/s speed=1.50x
frame=  100 fps= 45 q=28.0 size=    2048kB time=00:00:10.00 bitrate=1638.4kbits/s speed=2.00x
video:2048kB audio:0kB subtitle:0kB other streams:0kB global headers:0kB muxing overhead: unknown
`

	parser := NewParser(10 * time.Second)
	reader := strings.NewReader(input)

	var progressUpdates []Progress
	err := parser.Parse(reader, func(prog Progress) {
		progressUpdates = append(progressUpdates, prog)
	})

	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(progressUpdates) != 3 {
		t.Errorf("expected 3 progress updates, got %d", len(progressUpdates))
	}

	// Verify first update
	if len(progressUpdates) > 0 {
		first := progressUpdates[0]
		if first.Frame != 10 {
			t.Errorf("first update Frame = %d, want 10", first.Frame)
		}
		if first.Progress < 9.5 || first.Progress > 10.5 {
			t.Errorf("first update Progress = %.2f, want ~10", first.Progress)
		}
	}

	// Verify last update
	if len(progressUpdates) > 2 {
		last := progressUpdates[2]
		if last.Frame != 100 {
			t.Errorf("last update Frame = %d, want 100", last.Frame)
		}
		if last.Progress != 100.0 {
			t.Errorf("last update Progress = %.2f, want 100", last.Progress)
		}
	}
}

func TestEstimateETA(t *testing.T) {
	tests := []struct {
		name          string
		totalDuration time.Duration
		currentTime   time.Duration
		speed         float64
		wantETA       int // seconds
	}{
		{
			name:          "halfway at 1x speed",
			totalDuration: 100 * time.Second,
			currentTime:   50 * time.Second,
			speed:         1.0,
			wantETA:       50,
		},
		{
			name:          "halfway at 2x speed",
			totalDuration: 100 * time.Second,
			currentTime:   50 * time.Second,
			speed:         2.0,
			wantETA:       25,
		},
		{
			name:          "10% at 1.5x speed",
			totalDuration: 100 * time.Second,
			currentTime:   10 * time.Second,
			speed:         1.5,
			wantETA:       60,
		},
		{
			name:          "complete",
			totalDuration: 100 * time.Second,
			currentTime:   100 * time.Second,
			speed:         1.0,
			wantETA:       0,
		},
		{
			name:          "zero speed",
			totalDuration: 100 * time.Second,
			currentTime:   50 * time.Second,
			speed:         0.0,
			wantETA:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.totalDuration)
			got := parser.EstimateETA(tt.currentTime, tt.speed)

			if got != tt.wantETA {
				t.Errorf("EstimateETA() = %d, want %d", got, tt.wantETA)
			}
		})
	}
}
