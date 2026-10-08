package server

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"sitaw/internal/store"
)

var (
	// Item ids are client-generated UUIDs; they also appear in links (/i/<id>).
	idRe    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

const (
	maxCallsign = 24
	maxName     = 80
	maxRemarks  = 2000
	maxVertices = 2000
)

func cleanCallsign(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > maxCallsign {
		return "", errors.New("callsign must be 1-24 characters")
	}
	return s, nil
}

func validLatLon(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

func validateItem(it *store.Item) error {
	if !idRe.MatchString(it.ID) {
		return errors.New("bad id")
	}
	if it.UpdatedAt <= 0 {
		return errors.New("missing updatedAt")
	}
	if it.Deleted {
		return nil
	}
	if it.Folder != "" && !idRe.MatchString(it.Folder) {
		return errors.New("bad folder")
	}
	if it.Kind == store.KindFolder {
		it.Name = strings.TrimSpace(it.Name)
		if it.Name == "" || utf8.RuneCountInString(it.Name) > maxName || len(it.Coords) != 0 {
			return errors.New("folder needs a 1-80 character name and no coords")
		}
		if it.Color != "" && !colorRe.MatchString(it.Color) {
			return errors.New("bad color")
		}
		return nil
	}
	minPts := map[string]int{store.KindWaypoint: 1, store.KindLine: 2, store.KindArea: 3}[it.Kind]
	if minPts == 0 {
		return errors.New("bad kind")
	}
	if len(it.Coords) < minPts || len(it.Coords) > maxVertices || (it.Kind == store.KindWaypoint && len(it.Coords) != 1) {
		return errors.New("bad coords count")
	}
	for _, c := range it.Coords {
		if !validLatLon(c[0], c[1]) {
			return errors.New("bad coordinate")
		}
	}
	it.Name = strings.TrimSpace(it.Name)
	if utf8.RuneCountInString(it.Name) > maxName || utf8.RuneCountInString(it.Remarks) > maxRemarks {
		return errors.New("text too long")
	}
	if it.Color != "" && !colorRe.MatchString(it.Color) {
		return errors.New("bad color")
	}
	return nil
}
