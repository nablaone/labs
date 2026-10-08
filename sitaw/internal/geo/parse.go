package geo

import (
	"regexp"
	"strconv"
	"strings"
)

// Point is a parsed position. Precision is the rough size (m) of the
// referenced square/last digit; Kind is "mgrs" or "deg".
type Point struct {
	Lat, Lon  float64
	Precision float64
	Kind      string
}

// Parse accepts the same inputs as parseAnyCoord in coords.js: MGRS, or
// lat/lon in decimal degrees, degrees-minutes or degrees-minutes-seconds,
// with or without N/S/E/W. Without hemisphere letters the first value is
// latitude.
func Parse(input string) (Point, bool) {
	s := strings.TrimSpace(input)
	if s == "" {
		return Point{}, false
	}
	if lat, lon, p, ok := ParseMGRS(s); ok {
		return Point{lat, lon, p, "mgrs"}, true
	}
	if p, ok := parseDegrees(s); ok {
		p.Kind = "deg"
		return p, true
	}
	return Point{}, false
}

var (
	symbols   = strings.NewReplacer("°", " ", "º", " ", "˚", " ", "DEG", " ", `"`, " ", "″", " ", "”", " ", "''", " ", "'", " ", "′", " ", "’", " ")
	allowedRe = regexp.MustCompile(`^[0-9NSEW.,;\s+-]*$`)
	// Split points between number/letter tokens ("52.1N", "N52.1").
	letterDigit = regexp.MustCompile(`([NSEW])([\d+-])`)
	digitLetter = regexp.MustCompile(`(\d)([NSEW])`)
)

func isHemi(t string) bool { return len(t) == 1 && strings.ContainsAny(t, "NSEW") }

func tokens(s string) []string {
	s = letterDigit.ReplaceAllString(s, "$1 $2")
	s = digitLetter.ReplaceAllString(s, "$1 $2")
	return strings.Fields(s)
}

type angle struct {
	v         float64
	hemi      string
	precision float64
}

func parseDegrees(input string) (Point, bool) {
	s := symbols.Replace(strings.ToUpper(input))
	if !allowedRe.MatchString(s) {
		return Point{}, false
	}
	var parts []string
	if strings.ContainsAny(s, ",;") {
		parts = strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' })
		if strings.Count(s, ",")+strings.Count(s, ";") != 1 {
			return Point{}, false
		}
	} else {
		toks := tokens(s)
		if len(toks) == 0 {
			return Point{}, false
		}
		cut := -1
		if isHemi(toks[0]) {
			for i := 1; i < len(toks); i++ {
				if isHemi(toks[i]) {
					cut = i
					break
				}
			}
		} else {
			for i, t := range toks {
				if isHemi(t) {
					if i < len(toks)-1 {
						cut = i + 1
					}
					break
				}
			}
		}
		if cut < 0 {
			for _, t := range toks {
				if isHemi(t) {
					return Point{}, false
				}
			}
			if len(toks)%2 != 0 || len(toks) > 6 {
				return Point{}, false
			}
			cut = len(toks) / 2
		}
		parts = []string{strings.Join(toks[:cut], " "), strings.Join(toks[cut:], " ")}
	}
	if len(parts) != 2 {
		return Point{}, false
	}
	x, ok1 := parseAngle(parts[0])
	y, ok2 := parseAngle(parts[1])
	if !ok1 || !ok2 {
		return Point{}, false
	}
	isLat := func(h string) bool { return h == "N" || h == "S" }
	isLon := func(h string) bool { return h == "E" || h == "W" }
	lat, lon := x, y
	if isLon(x.hemi) || isLat(y.hemi) {
		lat, lon = y, x
	}
	if (lat.hemi != "" && !isLat(lat.hemi)) || (lon.hemi != "" && !isLon(lon.hemi)) {
		return Point{}, false
	}
	if lat.v < -90 || lat.v > 90 || lon.v < -180 || lon.v > 180 {
		return Point{}, false
	}
	return Point{Lat: lat.v, Lon: lon.v, Precision: min(x.precision, y.precision)}, true
}

var (
	firstNum = regexp.MustCompile(`^[+-]?\d+(\.\d+)?$`)
	otherNum = regexp.MustCompile(`^\d+(\.\d+)?$`)
)

func parseAngle(part string) (angle, bool) {
	var letters, nums []string
	for _, t := range tokens(part) {
		if strings.ContainsAny(t, "NSEW") {
			letters = append(letters, t)
		} else {
			nums = append(nums, t)
		}
	}
	if len(letters) > 1 || (len(letters) == 1 && !isHemi(letters[0])) || len(nums) < 1 || len(nums) > 3 {
		return angle{}, false
	}
	for i, n := range nums {
		re := otherNum
		if i == 0 {
			re = firstNum
		}
		if !re.MatchString(n) || (i < len(nums)-1 && strings.Contains(n, ".")) {
			return angle{}, false
		}
	}
	vals := []float64{0, 0, 0}
	for i, n := range nums {
		vals[i], _ = strconv.ParseFloat(strings.TrimLeft(n, "+-"), 64)
	}
	if vals[1] >= 60 || vals[2] >= 60 {
		return angle{}, false
	}
	neg := strings.HasPrefix(nums[0], "-")
	hemi := ""
	if len(letters) == 1 {
		hemi = letters[0]
	}
	if neg && hemi != "" {
		return angle{}, false
	}
	v := vals[0] + vals[1]/60 + vals[2]/3600
	if neg || hemi == "S" || hemi == "W" {
		v = -v
	}
	last := nums[len(nums)-1]
	decimals := 0
	if i := strings.IndexByte(last, '.'); i >= 0 {
		decimals = len(last) - i - 1
	}
	unit := []float64{111000, 1850, 31}[len(nums)-1]
	p := unit
	for range decimals {
		p /= 10
	}
	return angle{v: v, hemi: hemi, precision: max(1, p)}, true
}
