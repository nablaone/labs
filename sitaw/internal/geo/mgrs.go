// Package geo is the server-side twin of web/static/js/mgrs.js and coords.js
// (WGS84 <-> UTM <-> MGRS, coordinate parsing) plus measuring helpers for the
// agent API. Keep the two implementations in step: testdata/vectors.json is
// generated from the JS code and checked here.
package geo

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	a   = 6378137.0
	e2  = 0.00669438
	ep2 = e2 / (1 - e2)
	k0  = 0.9996
	d2r = math.Pi / 180
)

const (
	bands = "CDEFGHJKLMNPQRSTUVWXX"
	rows  = "ABCDEFGHJKLMNPQRSTUV"
)

var colSets = [3]string{"STUVWXYZ", "ABCDEFGH", "JKLMNPQR"} // by zone % 3

type UTM struct {
	Zone              int
	South             bool
	Easting, Northing float64
}

func UTMZone(lat, lon float64) int {
	lon = math.Mod(math.Mod(lon+180, 360)+360, 360) - 180
	zone := int(math.Floor((lon+180)/6)) + 1
	if zone > 60 {
		zone = 60
	}
	if lat >= 56 && lat < 64 && lon >= 3 && lon < 12 {
		zone = 32
	}
	if lat >= 72 && lat < 84 {
		switch {
		case lon >= 0 && lon < 9:
			zone = 31
		case lon >= 9 && lon < 21:
			zone = 33
		case lon >= 21 && lon < 33:
			zone = 35
		case lon >= 33 && lon < 42:
			zone = 37
		}
	}
	return zone
}

func centralMeridian(zone int) float64 { return float64((zone-1)*6 - 180 + 3) }

func latBand(lat float64) (byte, bool) {
	if lat < -80 || lat > 84 {
		return 0, false
	}
	return bands[min(int(math.Floor((lat+80)/8)), 20)], true
}

// ToUTM converts in the given zone (0 = the natural zone of the point).
func ToUTM(lat, lon float64, zone int) UTM {
	if zone == 0 {
		zone = UTMZone(lat, lon)
	}
	phi, lam, lam0 := lat*d2r, lon*d2r, centralMeridian(zone)*d2r
	sin, cos, tan := math.Sin(phi), math.Cos(phi), math.Tan(phi)
	N := a / math.Sqrt(1-e2*sin*sin)
	T := tan * tan
	C := ep2 * cos * cos
	A := cos * (lam - lam0)
	M := a * ((1-e2/4-3*e2*e2/64-5*e2*e2*e2/256)*phi -
		(3*e2/8+3*e2*e2/32+45*e2*e2*e2/1024)*math.Sin(2*phi) +
		(15*e2*e2/256+45*e2*e2*e2/1024)*math.Sin(4*phi) -
		(35*e2*e2*e2/3072)*math.Sin(6*phi))
	east := k0*N*(A+(1-T+C)*math.Pow(A, 3)/6+(5-18*T+T*T+72*C-58*ep2)*math.Pow(A, 5)/120) + 500000
	north := k0 * (M + N*tan*(A*A/2+(5-T+9*C+4*C*C)*math.Pow(A, 4)/24+(61-58*T+T*T+600*C-330*ep2)*math.Pow(A, 6)/720))
	if lat < 0 {
		north += 10000000
	}
	return UTM{Zone: zone, South: lat < 0, Easting: east, Northing: north}
}

func FromUTM(u UTM) (lat, lon float64) {
	e1 := (1 - math.Sqrt(1-e2)) / (1 + math.Sqrt(1-e2))
	x := u.Easting - 500000
	y := u.Northing
	if u.South {
		y -= 10000000
	}
	mu := y / k0 / (a * (1 - e2/4 - 3*e2*e2/64 - 5*e2*e2*e2/256))
	phi1 := mu + (3*e1/2-27*math.Pow(e1, 3)/32)*math.Sin(2*mu) +
		(21*e1*e1/16-55*math.Pow(e1, 4)/32)*math.Sin(4*mu) +
		(151*math.Pow(e1, 3)/96)*math.Sin(6*mu)
	sin, cos, tan := math.Sin(phi1), math.Cos(phi1), math.Tan(phi1)
	N1 := a / math.Sqrt(1-e2*sin*sin)
	T1 := tan * tan
	C1 := ep2 * cos * cos
	R1 := a * (1 - e2) / math.Pow(1-e2*sin*sin, 1.5)
	D := x / (N1 * k0)
	lat = phi1 - (N1*tan/R1)*(D*D/2-(5+3*T1+10*C1-4*C1*C1-9*ep2)*math.Pow(D, 4)/24+
		(61+90*T1+298*C1+45*T1*T1-252*ep2-3*C1*C1)*math.Pow(D, 6)/720)
	lon = (D - (1+2*T1+C1)*math.Pow(D, 3)/6 + (5-2*C1+28*T1-3*C1*C1+8*ep2+24*T1*T1)*math.Pow(D, 5)/120) / cos
	return lat / d2r, centralMeridian(u.Zone) + lon/d2r
}

func square100k(zone int, east, north float64) (string, bool) {
	ci := int(math.Floor(east/100000)) - 1
	set := colSets[zone%3]
	if ci < 0 || ci >= len(set) {
		return "", false
	}
	off := 0
	if zone%2 == 0 {
		off = 5
	}
	ri := (int(math.Floor(north/100000)) + off) % 20
	return string(set[ci]) + string(rows[ri]), true
}

// ToMGRS formats lat/lon as "34U DC 12345 67890" (digits per axis: 5 = 1 m).
// ok is false outside UTM coverage (polar regions).
func ToMGRS(lat, lon float64, digits int) (string, bool) {
	band, ok := latBand(lat)
	if !ok {
		return "", false
	}
	u := ToUTM(lat, lon, 0)
	sq, ok := square100k(u.Zone, u.Easting, u.Northing)
	if !ok {
		return "", false
	}
	zb := fmt.Sprintf("%02d%c", u.Zone, band)
	if digits == 0 {
		return zb + " " + sq, true
	}
	div := math.Pow(10, float64(5-digits))
	e := int(math.Floor(math.Mod(u.Easting, 100000) / div))
	n := int(math.Floor(math.Mod(u.Northing, 100000) / div))
	return fmt.Sprintf("%s %s %0*d %0*d", zb, sq, digits, e, digits, n), true
}

var mgrsRe = regexp.MustCompile(`^(\d{1,2})([C-HJ-NP-X])([A-HJ-NP-Z])([A-HJ-NP-V])(\d{0,10})$`)

// ParseMGRS returns the SW corner of the referenced square and its size in m.
func ParseMGRS(s string) (lat, lon, precision float64, ok bool) {
	s = strings.ToUpper(strings.Join(strings.Fields(s), ""))
	m := mgrsRe.FindStringSubmatch(s)
	if m == nil || len(m[5])%2 != 0 {
		return 0, 0, 0, false
	}
	zone, _ := strconv.Atoi(m[1])
	if zone < 1 || zone > 60 {
		return 0, 0, 0, false
	}
	col := strings.IndexByte(colSets[zone%3], m[3][0])
	row := strings.IndexByte(rows, m[4][0])
	if col < 0 || row < 0 {
		return 0, 0, 0, false
	}
	if zone%2 == 0 {
		row = (row - 5 + 20) % 20
	}
	half := len(m[5]) / 2
	precision = 100000
	var e, n float64
	if half > 0 {
		precision = math.Pow(10, float64(5-half))
		ev, _ := strconv.Atoi(m[5][:half])
		nv, _ := strconv.Atoi(m[5][half:])
		e, n = float64(ev)*precision, float64(nv)*precision
	}
	bandLat := -80 + float64(strings.IndexByte(bands, m[2][0]))*8
	south := bandLat < 0
	minN := ToUTM(bandLat, centralMeridian(zone), zone).Northing - 100000
	north := float64(row)*100000 + n
	for north < minN {
		north += 2000000
	}
	lat, lon = FromUTM(UTM{Zone: zone, South: south, Easting: float64(col+1)*100000 + e, Northing: north})
	if math.IsNaN(lat) || math.IsNaN(lon) {
		return 0, 0, 0, false
	}
	return lat, lon, precision, true
}
