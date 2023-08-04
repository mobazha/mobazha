import app from '../app';
import { version } from '../../../../package.json';
import { recordEvent, userStats } from './metrics';


/*
 * Sets the options for the feedback tool
 */

function setFeedbackOptions() {
  const profile = app.profile ? app.profile.toJSON() : {};
  const contactInfo = profile.contactInfo || {};
  const sVer = app.settings && app.settings.get('version') || '';
  const serverVersion = sVer.substring(sVer.lastIndexOf(':') + 1, sVer.lastIndexOf('/'));

  window.doorbellOptions = {
    id: 9640,
    appKey: "ua0oMkpckoyRJxdN309uk2o2G35MKLUSJCpPNdN4ezDBVl4qWvub2vDLfdJQcVc8",
    name: profile.name || 'profile data missing',
    onShow: () => recordEvent('Feedback_Show'),
    onError: (code, message) => recordEvent('Feedback_Error', { code, message }),
    properties: {
      settingsReadable: !!app.settings,
      profileReadable: !!app.profile,
      peerID: profile.peerID || 'profile data missing',
      clientVersion: version,
      serverVersion,
      contactInfo,
      ...userStats(),
    },
  };
}

/*
 * Adds the feedback tool to the page.
 */
export function addFeedback() {
  if (!window.doorbellOptions) setFeedbackOptions();

  (function (w, d, t) {
    let hasLoaded = false;
    function l() {
      if (hasLoaded) {
        return;
      }
      hasLoaded = true;
      window.doorbellOptions.windowLoaded = true;
      const g = d.createElement(t);
      g.id = 'doorbellScript';
      g.type = 'text/javascript';
      g.async = true;
      g.src = `https://embed.doorbell.io/button/9640?t=${(new Date().getTime())}`;
      (d.getElementsByTagName('head')[0] || d.getElementsByTagName('body')[0]).appendChild(g);
    }
    if (w.attachEvent) {
      w.attachEvent('onload', l);
    } else if (w.addEventListener) {
      w.addEventListener('load', l, false);
    } else {
      l();
    }
    if (d.readyState === 'complete') {
      l();
    }
  }(window, document, 'script'));
}
