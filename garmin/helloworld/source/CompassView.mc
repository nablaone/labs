import Toybox.Graphics;
import Toybox.Lang;
import Toybox.Math;
import Toybox.Sensor;
import Toybox.WatchUi;

class CompassView extends WatchUi.View {

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
        var cy = h / 2 - 8;

        var info    = Sensor.getInfo();
        var heading = (info != null && info.heading != null)
            ? (info.heading as Float).toDouble()
            : null;

        _drawFace(dc, cx, cy);

        if (heading == null) {
            dc.drawText(cx, cy, Graphics.FONT_SMALL, "---",
                Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER);
            dc.drawText(cx, h - 16, Graphics.FONT_XTINY, "No compass data",
                Graphics.TEXT_JUSTIFY_CENTER);
            return;
        }

        _drawNeedle(dc, cx, cy, heading as Double);

        var deg = ((heading as Double) * 180.0d / Math.PI).toNumber();
        if (deg < 0) { deg += 360; }
        dc.drawText(cx, h - 22, Graphics.FONT_MEDIUM, deg.format("%03d") + "\u00b0",
            Graphics.TEXT_JUSTIFY_CENTER);
    }

    private function _drawFace(dc as Dc, cx as Number, cy as Number) as Void {
        var R = 68;
        dc.drawCircle(cx, cy, R);

        // Cardinal angles and labels
        var cardDeg  = [0,   90,  180, 270] as Array<Number>;
        var cardLbls = ["N", "E", "S", "W"] as Array<String>;

        // Minor ticks at non-cardinal 30° steps
        var minorDeg = [30, 60, 120, 150, 210, 240, 300, 330] as Array<Number>;
        for (var i = 0; i < minorDeg.size(); i++) {
            var rad  = minorDeg[i].toDouble() * Math.PI / 180.0d;
            var sinR = Math.sin(rad).toFloat();
            var cosR = Math.cos(rad).toFloat();
            dc.drawLine(
                (cx + R * sinR).toNumber(),       (cy - R * cosR).toNumber(),
                (cx + (R - 8) * sinR).toNumber(), (cy - (R - 8) * cosR).toNumber()
            );
        }

        // Cardinal ticks and labels
        for (var i = 0; i < cardDeg.size(); i++) {
            var rad  = cardDeg[i].toDouble() * Math.PI / 180.0d;
            var sinR = Math.sin(rad).toFloat();
            var cosR = Math.cos(rad).toFloat();
            dc.drawLine(
                (cx + R * sinR).toNumber(),        (cy - R * cosR).toNumber(),
                (cx + (R - 14) * sinR).toNumber(), (cy - (R - 14) * cosR).toNumber()
            );
            dc.drawText(
                (cx + (R - 26) * sinR).toNumber(),
                (cy - (R - 26) * cosR).toNumber(),
                Graphics.FONT_XTINY, cardLbls[i],
                Graphics.TEXT_JUSTIFY_CENTER | Graphics.TEXT_JUSTIFY_VCENTER
            );
        }
    }

    private function _drawNeedle(dc as Dc, cx as Number, cy as Number, heading as Double) as Void {
        var len  = 52;
        var half = 9;

        var sinH = Math.sin(heading).toFloat();
        var cosH = Math.cos(heading).toFloat();

        var nTipX = (cx + len  *  sinH).toNumber();
        var nTipY = (cy - len  *  cosH).toNumber();
        var sTipX = (cx - len  *  sinH).toNumber();
        var sTipY = (cy + len  *  cosH).toNumber();
        var lX    = (cx + half *  cosH).toNumber();
        var lY    = (cy + half *  sinH).toNumber();
        var rX    = (cx - half *  cosH).toNumber();
        var rY    = (cy - half *  sinH).toNumber();

        // North half — filled white
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.fillPolygon([[nTipX, nTipY], [lX, lY], [rX, rY]]);

        // South half — black fill, white outline
        dc.setColor(Graphics.COLOR_BLACK, Graphics.COLOR_BLACK);
        dc.fillPolygon([[sTipX, sTipY], [lX, lY], [rX, rY]]);
        dc.setColor(Graphics.COLOR_WHITE, Graphics.COLOR_BLACK);
        dc.drawLine(sTipX, sTipY, lX, lY);
        dc.drawLine(sTipX, sTipY, rX, rY);
        dc.drawLine(lX, lY, rX, rY);

        // Centre dot
        dc.fillCircle(cx, cy, 4);
    }

}
