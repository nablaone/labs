import Toybox.Application;
import Toybox.Lang;
import Toybox.Position;
import Toybox.WatchUi;

class GpsApp extends Application.AppBase {

    private var mPosition as Position.Info?;

    function initialize() {
        AppBase.initialize();
    }

    function onStart(state as Dictionary?) as Void {
        Position.enableLocationEvents(Position.LOCATION_CONTINUOUS, method(:onPosition));
    }

    function onStop(state as Dictionary?) as Void {
        Position.enableLocationEvents(Position.LOCATION_DISABLE, null);
    }

    function onPosition(info as Position.Info) as Void {
        mPosition = info;
        WatchUi.requestUpdate();
    }

    function getPosition() as Position.Info? {
        return mPosition;
    }

    function getInitialView() as [Views] or [Views, InputDelegates] {
        var delegate = new AppDelegate();
        return [delegate.getCurrentView(), delegate];
    }

}

function getApp() as GpsApp {
    return Application.getApp() as GpsApp;
}
