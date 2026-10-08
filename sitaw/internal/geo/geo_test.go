package geo

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// vectors.json comes from the browser code (web/tools/gen-geo-vectors.mjs);
// the server must agree with the client exactly.
type vectors struct {
	ToMgrs []struct {
		Lat, Lon float64
		Digits   int
		MGRS     *string `json:"mgrs"`
	} `json:"toMgrs"`
	FromMgrs []struct {
		MGRS                string `json:"mgrs"`
		Lat, Lon, Precision float64
	} `json:"fromMgrs"`
	Parse []struct {
		Input  string `json:"input"`
		Result *struct {
			Lat, Lon, Precision float64
			Kind                string
		} `json:"result"`
	} `json:"parse"`
}

func load(t *testing.T) vectors {
	b, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestToMGRSMatchesClient(t *testing.T) {
	for _, v := range load(t).ToMgrs {
		got, ok := ToMGRS(v.Lat, v.Lon, v.Digits)
		want := ""
		if v.MGRS != nil {
			want = *v.MGRS
		}
		if (v.MGRS != nil) != ok || got != want {
			t.Fatalf("ToMGRS(%v, %v, %d) = %q %v, client %q", v.Lat, v.Lon, v.Digits, got, ok, want)
		}
	}
}

func TestParseMGRSMatchesClient(t *testing.T) {
	for _, v := range load(t).FromMgrs {
		lat, lon, p, ok := ParseMGRS(v.MGRS)
		if !ok || math.Abs(lat-v.Lat) > 1e-9 || math.Abs(lon-v.Lon) > 1e-9 || p != v.Precision {
			t.Fatalf("ParseMGRS(%q) = %v %v %v %v, client %v %v %v", v.MGRS, lat, lon, p, ok, v.Lat, v.Lon, v.Precision)
		}
	}
}

func TestParseMatchesClient(t *testing.T) {
	for _, v := range load(t).Parse {
		got, ok := Parse(v.Input)
		if v.Result == nil {
			if ok {
				t.Errorf("Parse(%q) = %+v, client: not a coordinate", v.Input, got)
			}
			continue
		}
		r := v.Result
		if !ok || math.Abs(got.Lat-r.Lat) > 1e-9 || math.Abs(got.Lon-r.Lon) > 1e-9 || got.Kind != r.Kind || math.Abs(got.Precision-r.Precision) > 1e-9 {
			t.Errorf("Parse(%q) = %+v %v, client %+v", v.Input, got, ok, *r)
		}
	}
}

func TestMeasure(t *testing.T) {
	// Warsaw Palace of Culture -> Kraków Main Square: ~252 km, bearing ~195°.
	d := Distance(52.2318, 21.0060, 50.0617, 19.9373)
	if math.Abs(d-252000) > 3000 {
		t.Fatalf("distance %v", d)
	}
	if b := Bearing(52.2318, 21.0060, 50.0617, 19.9373); math.Abs(b-196.5) > 2 {
		t.Fatalf("bearing %v", b)
	}
	if b := Bearing(0, 0, 1, 0); math.Abs(b) > 1e-9 {
		t.Fatalf("north %v", b)
	}
	// Central meridian of zone 34 is 21°E: no convergence there, positive east of it.
	if c := GridConvergence(52, 21); math.Abs(c) > 1e-9 {
		t.Fatalf("convergence on CM %v", c)
	}
	if c := GridConvergence(52, 23); c <= 0 || c > 2 {
		t.Fatalf("convergence east of CM %v", c)
	}
	square := [][2]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}
	if !InPolygon(0.5, 0.5, square) || InPolygon(1.5, 0.5, square) {
		t.Fatal("point in polygon")
	}
	// 0.5° north of a segment along the equator: ~55.6 km.
	if d := DistanceToPath(0.5, 0.5, [][2]float64{{0, 0}, {0, 1}}, false); math.Abs(d-55600) > 300 {
		t.Fatalf("distance to path %v", d)
	}
}
