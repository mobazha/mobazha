import Countly from 'countly-sdk-react-native-bridge';

export default class EventTracker {
  constructor(isTracking = false) {
    this.isTracking = isTracking;
  }

  changeTrackingStatus(isTracking) {
    if (!isTracking) {
      if(this.isTracking) {
        Countly.stop();
      }
    } else {
      if(!this.isTracking) {
        Countly.start();
      }
    }
    this.isTracking = isTracking;
  }

  getTrackingStatus() {
    return this.isTracking;
  }

  asyncEventStart(event) {
    if (this.isTracking) {
      Countly.startEvent(event);
    }
  }

  asyncEventFinished(event) {
    if (this.isTracking) {
      Countly.endEvent(event);
    }
  }

  asyncEventSuccess(event) {
    if (this.isTracking) {
      Countly.recordEvent({ key: 'AsyncEventSuccess', segmentation: { event } });
    }
  }

  trackEvent(key, segmentation = {}) {
    if (this.isTracking) {
      Countly.recordEvent({ key, segmentation });
    }
  }
}

