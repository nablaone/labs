import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Position;
import Toybox.WatchUi;

class DegreesView extends WatchUi.View {

    function initialize() {
        View.initialize();
    }

    function onLayout(dc as Dc) as Void {}
    function onShow()  as Void {}
    function onHide()  as Void {}

    function onUpdate(dc as Dc) as Void {
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.clear();

        var w  = dc.getWidth();
        var h  = dc.getHeight();
        var cx = w / 2;
        var cy = h / 2;

        // Header
        dc.drawText(cx, 8, Graphics.FONT_SMALL, "COORDINATES", Graphics.TEXT_JUSTIFY_CENTER);
        dc.drawLine(0, 26, w, 26);

        var pos = getApp().getPosition();
        if (pos == null || pos.accuracy == Position.QUALITY_NOT_AVAILABLE) {
            dc.drawText(cx, cy, Graphics.FONT_SMALL, "Acquiring GPS...",
                Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);
            return;
        }

        var coords = pos.position.toDegrees();
        var strs   = GeoHelper.toDecimalDegrees(coords[0].toDouble(), coords[1].toDouble());

        // Latitude
        dc.drawText(cx, cy - 20, Graphics.FONT_MEDIUM, strs[0],
            Graphics.TEXT_JUSTIFY_CENTER);
        // Longitude
        dc.drawText(cx, cy + 16, Graphics.FONT_MEDIUM, strs[1],
            Graphics.TEXT_JUSTIFY_CENTER);

        // Accuracy
        var accStr = _accuracyLabel(pos.accuracy);
        dc.drawText(cx, h - 16, Graphics.FONT_XTINY, accStr,
            Graphics.TEXT_JUSTIFY_CENTER);
    }

    private function _accuracyLabel(acc as Position.Quality) as String {
        if (acc == Position.QUALITY_GOOD)       { return "GPS: good"; }
        if (acc == Position.QUALITY_USABLE)     { return "GPS: ok"; }
        if (acc == Position.QUALITY_POOR)       { return "GPS: poor"; }
        if (acc == Position.QUALITY_LAST_KNOWN) { return "GPS: last known"; }
        return "GPS: none";
    }

}
