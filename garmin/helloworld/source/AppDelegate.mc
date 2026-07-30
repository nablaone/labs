import Toybox.Lang;
import Toybox.WatchUi;

class AppDelegate extends WatchUi.BehaviorDelegate {

    private var mViews as Array;
    private var mIdx   as Number = 0;

    function initialize() {
        BehaviorDelegate.initialize();
        mViews = [
            new MGRSView(),
            new DegreesView(),
            new CompassView()
        ] as Array;
    }

    function getCurrentView() as WatchUi.View {
        return mViews[mIdx] as WatchUi.View;
    }

    function onNextPage() as Boolean {
        mIdx = (mIdx + 1) % 3;
        WatchUi.switchToView(mViews[mIdx] as WatchUi.View, self, WatchUi.SLIDE_UP);
        return true;
    }

    function onPreviousPage() as Boolean {
        mIdx = (mIdx + 2) % 3;
        WatchUi.switchToView(mViews[mIdx] as WatchUi.View, self, WatchUi.SLIDE_DOWN);
        return true;
    }

}
