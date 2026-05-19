// Package trail implements the core data structures and logic for
// vaporwave-style motion trails: a ring buffer of timestamped points
// and gradient color interpolation.
package trail

import (
	"math"
	"time"

	"vapormove/config"
)

// Point is a single captured position with a timestamp
type Point struct {
	X, Y     float32
	W, H     float32 // size for window trail
	CapturedAt time.Time
}

// GhostPoint is a point ready for rendering, with an interpolated color
// and an opacity value derived from its age
type GhostPoint struct {
	X, Y    float32
	W, H    float32 // size for window trail
	R, G, B float32
	A       float32 // pre-multiplied with gradient alpha and age fade
}

// Trail is a fixed-capacity ring buffer of Points
type Trail struct {
	points   []Point
	head     int // index of the next write slot
	count    int // number of valid points
	capacity int
	fadeMs   float64
	gradient []config.GradientStop
}

// New creates a Trail with the given config
func New(cfg config.TrailConfig) *Trail {
	cap := cfg.Length
	if cap < 1 {
		cap = 1
	}
	return &Trail{
		points:   make([]Point, cap),
		capacity: cap,
		fadeMs:   float64(cfg.FadeMs),
		gradient: cfg.Gradient,
	}
}

// Push adds a new point to the trail, evicting the oldest if full
func (t *Trail) Push(x, y float32) {
	t.points[t.head] = Point{X: x, Y: y, CapturedAt: time.Now()}
	t.head = (t.head + 1) % t.capacity
	if t.count < t.capacity {
		t.count++
	}
}

// PushWithSize adds a point with size (for window trail)
func (t *Trail) PushWithSize(x, y, w, h float32) {
	t.points[t.head] = Point{X: x, Y: y, W: w, H: h, CapturedAt: time.Now()}
	t.head = (t.head + 1) % t.capacity
	if t.count < t.capacity {
		t.count++
	}
}

// Snapshot returns a slice of GhostPoints ordered from newest (index 0)
// to oldest, ready for rendering. Points older than fadeMs are excluded.
func (t *Trail) Snapshot() []GhostPoint {
	if t.count == 0 {
		return nil
	}

	now := time.Now()
	result := make([]GhostPoint, 0, t.count)

	for i := 0; i < t.count; i++ {
		// Walk backwards from head: i=0 is newest
		idx := (t.head - 1 - i + t.capacity) % t.capacity
		p := t.points[idx]

		ageMs := float64(now.Sub(p.CapturedAt).Milliseconds())
		if ageMs >= t.fadeMs {
			break // older points are also expired
		}

		// t=0.0 at tip (newest), t=1.0 at tail (oldest)
		tPos := float32(i) / float32(t.count-1+1)

		// Age-based opacity: linear fade over fadeMs
		ageFade := float32(1.0 - ageMs/t.fadeMs)

		r, g, b, a := t.interpolateGradient(tPos)
		result = append(result, GhostPoint{
			X: p.X,
			Y: p.Y,
			W: p.W,
			H: p.H,
			R: r,
			G: g,
			B: b,
			A: a * ageFade,
		})
	}

	return result
}

// interpolateGradient returns RGBA for a gradient position t in [0,1]
func (t *Trail) interpolateGradient(pos float32) (r, g, b, a float32) {
	stops := t.gradient
	if len(stops) == 0 {
		return 0.5, 0.1, 1.0, 0.8
	}
	if len(stops) == 1 {
		c := stops[0].Color
		return c.R, c.G, c.B, c.A
	}

	// Clamp
	if pos <= stops[0].Position {
		c := stops[0].Color
		return c.R, c.G, c.B, c.A
	}
	if pos >= stops[len(stops)-1].Position {
		c := stops[len(stops)-1].Color
		return c.R, c.G, c.B, c.A
	}

	// Find surrounding stops
	for i := 1; i < len(stops); i++ {
		if pos <= stops[i].Position {
			lo := stops[i-1]
			hi := stops[i]
			span := hi.Position - lo.Position
			if span <= 0 {
				c := hi.Color
				return c.R, c.G, c.B, c.A
			}
			f := (pos - lo.Position) / span
			f = smoothstep(f) // ease in-out for nicer gradient
			return lerp(lo.Color.R, hi.Color.R, f),
				lerp(lo.Color.G, hi.Color.G, f),
				lerp(lo.Color.B, hi.Color.B, f),
				lerp(lo.Color.A, hi.Color.A, f)
		}
	}

	c := stops[len(stops)-1].Color
	return c.R, c.G, c.B, c.A
}

// UpdateConfig replaces the gradient and fade settings without clearing points
func (t *Trail) UpdateConfig(cfg config.TrailConfig) {
	t.fadeMs = float64(cfg.FadeMs)
	t.gradient = cfg.Gradient
}

// Clear discards all stored points
func (t *Trail) Clear() {
	t.head = 0
	t.count = 0
}

// ── Math helpers ──────────────────────────────────────────────────────

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

func smoothstep(t float32) float32 {
	// Ken Perlin's smoothstep
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// Distance returns Euclidean distance between two points
func Distance(ax, ay, bx, by float32) float32 {
	dx := float64(ax - bx)
	dy := float64(ay - by)
	return float32(math.Sqrt(dx*dx + dy*dy))
}
