// Putting start-up related one offs here that are too small for their own module and
// aren't appropriate to be in any existing module

import { Renderer, ipc } from '../../src/utils/ipcRenderer.js';
import { getBody } from '../utils/selectors';
import { getCurrentConnection } from '../utils/serverConnect';
import app from '../app';
import { myPost } from '../../src/api/api.js';

const platform = ipc.sendSync('controller.system.getPlatform', {});

export function fixLinuxZoomIssue() {
  // fix zoom issue on Linux hiDPI
  if (platform === 'linux') {
    try {
      let { scaleFactor } = Renderer.screen.getPrimaryDisplay();

      if (scaleFactor === 0) {
        scaleFactor = 1;
      }

      getBody().css('zoom', 1 / scaleFactor);
    } catch (e) {
      console.error('Unable to fix the linux zoom issue due to an error.');
      console.error(e);
    }
  }
}

/**
 * This function will accept requests from the main process to shutdown the OB server daemon.
 * This should only be called on the bundled app on windows. For Linux and OSX, the localServer
 * module is able to shut down the daemon via OS signals.
 */
export function handleServerShutdownRequests() {
  ipc.on('server-shutdown', () => {
    if (platform !== 'win32') {
      ipc.send(
        'server-shutdown-fail',
        { reason: 'Not on windows. Use childProcess.kill instead.' },
      );
      return;
    }

    const curConn = getCurrentConnection();

    if (!curConn || curConn.status !== 'connected') {
      ipc.send(
        'server-shutdown-fail',
        { reason: 'No server connection' },
      );
      return;
    }

    try {
      myPost(app.getServerUrl('ob/shutdown'))
        .fail((xhr) => ipc.send('server-shutdown-fail', {
          xhr,
          reason: (xhr && xhr.responseJSON && xhr.responseJSON.reason) || '',
        }));
    } catch (e) {
      ipc.send(
        'server-shutdown-fail',
        { reason: e.toString() },
      );
    }
  });
}
