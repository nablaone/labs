import Toybox.Lang;
import Toybox.Math;

// WGS84 UTM / MGRS conversion utilities
module GeoHelper {

    // WGS84 constants
    const A  = 6378137.0d;
    const F  = 1.0d / 298.257223563d;
    const K0 = 0.9996d;

    // Returns ["18T", "WL", "12345 67890"] or ["???", "??", "?????  ?????"]
    function toMGRS(lat as Double, lon as Double) as Array<String> {
        if (lat < -80.0d || lat > 84.0d) {
            return ["???", "??", "????? ?????"] as Array<String>;
        }

        var zoneNum = ((lon + 180.0d) / 6.0d).toNumber() + 1;
        if (zoneNum > 60) { zoneNum = 60; }
        var zoneLetter = _zoneLetter(lat);

        var utm = _toUTM(lat, lon, zoneNum);
        var easting  = utm[0];
        var northing = utm[1];

        // 100km column letter (easting-based)
        var colSets = ["ABCDEFGH", "JKLMNPQR", "STUVWXYZ"] as Array<String>;
        var colIdx  = (easting / 100000.0d).toNumber() - 1;
        if (colIdx < 0) { colIdx = 0; }
        if (colIdx > 7) { colIdx = 7; }
        var colSet  = (zoneNum - 1) % 3;
        var colLtr  = colSets[colSet].substring(colIdx, colIdx + 1);

        // 100km row letter (northing-based, cycles every 2000km)
        var rowOdd  = "ABCDEFGHJKLMNPQRSTUV";
        var rowEven = "FGHJKLMNPQRSTUVABCDE";
        var rowLtrs = (zoneNum % 2 == 1) ? rowOdd : rowEven;
        var rowIdx  = (northing / 100000.0d).toNumber() % 20;
        var rowLtr  = rowLtrs.substring(rowIdx, rowIdx + 1);

        var eNum = (easting  - ((easting  / 100000.0d).toNumber().toDouble() * 100000.0d)).toNumber();
        var nNum = (northing - ((northing / 100000.0d).toNumber().toDouble() * 100000.0d)).toNumber();

        return [
            zoneNum.format("%d") + zoneLetter,
            colLtr + rowLtr,
            eNum.format("%05d") + " " + nNum.format("%05d")
        ] as Array<String>;
    }

    // Returns decimal degrees as ["40.712776° N", "-74.005974° W"]
    function toDecimalDegrees(lat as Double, lon as Double) as Array<String> {
        var latDir = (lat >= 0.0d) ? "N" : "S";
        var lonDir = (lon >= 0.0d) ? "E" : "W";
        return [
            lat.format("%.5f") + "\u00b0 " + latDir,
            lon.format("%.5f") + "\u00b0 " + lonDir
        ] as Array<String>;
    }

    // --- private helpers ---

    function _zoneLetter(lat as Double) as String {
        var bands = "CDEFGHJKLMNPQRSTUVWX";
        var idx = ((lat + 80.0d) / 8.0d).toNumber();
        if (idx < 0)  { idx = 0; }
        if (idx > 19) { idx = 19; }
        return bands.substring(idx, idx + 1);
    }

    // Returns [easting, northing] in metres (WGS84 UTM)
    function _toUTM(lat as Double, lon as Double, zoneNum as Number) as Array<Double> {
        var b      = A * (1.0d - F);
        var eccSq  = 1.0d - (b / A) * (b / A);
        var eccPSq = eccSq / (1.0d - eccSq);   // e prime squared

        var latR  = lat * Math.PI / 180.0d;
        var lonR  = lon * Math.PI / 180.0d;
        var lon0R = ((zoneNum - 1) * 6 - 180 + 3).toDouble() * Math.PI / 180.0d;

        var sinLat = Math.sin(latR);
        var cosLat = Math.cos(latR);
        var tanLat = Math.tan(latR);

        var N  = A / Math.sqrt(1.0d - eccSq * sinLat * sinLat);
        var T  = tanLat * tanLat;
        var C  = eccPSq * cosLat * cosLat;
        var av = cosLat * (lonR - lon0R);   // A in standard formula
        var av2 = av * av;
        var av3 = av2 * av;
        var av4 = av3 * av;
        var av5 = av4 * av;
        var av6 = av5 * av;

        var M = A * (
            (1.0d - eccSq/4.0d - 3.0d*eccSq*eccSq/64.0d - 5.0d*eccSq*eccSq*eccSq/256.0d) * latR
            - (3.0d*eccSq/8.0d + 3.0d*eccSq*eccSq/32.0d + 45.0d*eccSq*eccSq*eccSq/1024.0d) * Math.sin(2.0d * latR)
            + (15.0d*eccSq*eccSq/256.0d + 45.0d*eccSq*eccSq*eccSq/1024.0d) * Math.sin(4.0d * latR)
            - (35.0d*eccSq*eccSq*eccSq/3072.0d) * Math.sin(6.0d * latR)
        );

        var easting = K0 * N * (
            av
            + (1.0d - T + C) * av3 / 6.0d
            + (5.0d - 18.0d*T + T*T + 72.0d*C - 58.0d*eccPSq) * av5 / 120.0d
        ) + 500000.0d;

        var northing = K0 * (
            M + N * tanLat * (
                av2 / 2.0d
                + (5.0d - T + 9.0d*C + 4.0d*C*C) * av4 / 24.0d
                + (61.0d - 58.0d*T + T*T + 600.0d*C - 330.0d*eccPSq) * av6 / 720.0d
            )
        );

        if (lat < 0.0d) {
            northing = northing + 10000000.0d;
        }

        return [easting, northing] as Array<Double>;
    }

}
