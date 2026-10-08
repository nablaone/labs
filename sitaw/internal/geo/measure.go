package geo

import "math"

const earthR = 6371008.8 // mean radius, m

// Distance is the great-circle distance in meters (haversine; within ~0.5%
// of the ellipsoidal value, fine for situational awareness).
func Distance(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*d2r, lat2*d2r
	dp, dl := (lat2-lat1)*d2r, (lon2-lon1)*d2r
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * earthR * math.Asin(math.Min(1, math.Sqrt(h)))
}

// Bearing is the initial true bearing from point 1 to point 2, degrees [0,360).
func Bearing(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2, dl := lat1*d2r, lat2*d2r, (lon2-lon1)*d2r
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	return math.Mod(math.Atan2(y, x)/d2r+360, 360)
}

// GridConvergence is the angle (degrees) between grid north and true north
// at a point, in its UTM zone. Grid bearing = true bearing - convergence.
func GridConvergence(lat, lon float64) float64 {
	dl := (lon - centralMeridian(UTMZone(lat, lon))) * d2r
	return math.Atan(math.Tan(dl)*math.Sin(lat*d2r)) / d2r
}

// InPolygon reports whether (lat, lon) is inside the ring (ray casting on
// lat/lon; fine for the small areas drawn in the app).
func InPolygon(lat, lon float64, ring [][2]float64) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		yi, xi := ring[i][0], ring[i][1]
		yj, xj := ring[j][0], ring[j][1]
		if (yi > lat) != (yj > lat) && lon < (xj-xi)*(lat-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// DistanceToPath is the shortest distance (m) from a point to a polyline
// (or polygon outline if closed), using a local flat projection per segment.
func DistanceToPath(lat, lon float64, pts [][2]float64, closed bool) float64 {
	if len(pts) == 1 {
		return Distance(lat, lon, pts[0][0], pts[0][1])
	}
	best := math.Inf(1)
	n := len(pts) - 1
	if closed {
		n = len(pts)
	}
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%len(pts)]
		best = math.Min(best, distToSegment(lat, lon, a, b))
	}
	return best
}

func distToSegment(lat, lon float64, a, b [2]float64) float64 {
	// Equirectangular projection around the query point.
	k := math.Cos(lat * d2r)
	ax, ay := (a[1]-lon)*k, a[0]-lat
	bx, by := (b[1]-lon)*k, b[0]-lat
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
	}
	px, py := ax+t*dx, ay+t*dy
	return Distance(lat, lon, lat+py, lon+px/k)
}
