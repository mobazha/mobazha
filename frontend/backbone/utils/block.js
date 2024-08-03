import { Model, Events } from 'backbone';
import * as isIPFS from 'is-ipfs';
import { myAjax } from '../../src/api/api';
import { openSimpleMessage } from '../views/modals/SimpleMessage';
import app from '../app';

const events = {
  ...Events,
};

export { events };

function checkAppSettings() {
  if (!app || !(app.settings instanceof Model)) {
    throw new Error('app.settings must be a model.');
  }

  if (!Array.isArray(app.settings.get('blockedNodes'))) {
    throw new Error('app.settings.blockedNodes must be set as an array.');
  }
}

let latestSettingsSave;
let lastSentBlockedNodes = [];
let pendingBlocks = [];
let pendingUnblocks = [];

// FYI - The extra complexity in this method is due to the fact that for the block API
// you are sending the full list rather than individual items you want to unblock / unblock
// and that gets interesting if you kick off a subsequent request while a previous is still
// pending and, for example, the previous may fail whereas the subsequent (which includes
// the block/unblock from the previous) may succeed.
function blockUnblock(_block, peerIDs) {
  if (typeof _block !== 'boolean') {
    throw new Error('Please provide _block as a boolean.');
  }

  if (!isIPFS.multihash(peerIDs) && !Array.isArray(peerIDs)) {
    throw new Error('Either provide a single peerID as a multihash or an array of peerID multihashes.');
  }

  if (Array.isArray(peerIDs)) {
    peerIDs.forEach((peerID) => {
      if (!isIPFS.multihash(peerID)) {
        throw new Error('If providing an array of peerIDs, each item must be a multihash.');
      }
    });
  }

  checkAppSettings();

  let peerIDList = typeof peerIDs === 'string' ? [peerIDs] : peerIDs;

  if (_block && app.profile && peerIDList.includes(app.profile.id)) {
    throw new Error('You cannot block your own node.');
  }

  // de-dupe peerID list
  peerIDList = Array.from(new Set(peerIDList));

  let blockedNodes; // if _block is false, semantically this means unblockedNodes

  if (_block) {
    blockedNodes = [
      ...(
        latestSettingsSave && latestSettingsSave.state() === 'pending'
          ? lastSentBlockedNodes : app.settings.get('blockedNodes')
      ),
      ...peerIDList,
    ];
    pendingBlocks = [...pendingBlocks, ...peerIDList];
    pendingUnblocks = pendingUnblocks.filter((peerID) => !peerIDList.includes(peerID));
  } else {
    const filterList = latestSettingsSave && latestSettingsSave.state() === 'pending'
      ? lastSentBlockedNodes : app.settings.get('blockedNodes');
    blockedNodes = filterList.filter((peerID) => !peerIDList.includes(peerID));
    pendingUnblocks = [...pendingUnblocks, ...peerIDList];
    pendingBlocks = pendingBlocks.filter((peerID) => !peerIDList.includes(peerID));
  }

  lastSentBlockedNodes = [...blockedNodes];

  latestSettingsSave = myAjax({
    type: 'PUT',
    url: app.getServerUrl('ob/preferences'),
    data: JSON.stringify({ blockedNodes }),
    dataType: 'json',
  }).done(() => {
    app.settings.set('blockedNodes', blockedNodes);
    const blocked = [];
    const unblocked = [];

    pendingBlocks = pendingBlocks.filter((peerID) => {
      if (blockedNodes.includes(peerID)) {
        blocked.push(peerID);
        return false;
      }

      return true;
    });

    pendingUnblocks = pendingUnblocks.filter((peerID) => {
      if (!blockedNodes.includes(peerID)) {
        unblocked.push(peerID);
        return false;
      }

      return true;
    });

    if (blocked.length) {
      events.trigger('blocked', { peerIDs: blocked });
    }

    if (unblocked.length) {
      events.trigger('unblocked', { peerIDs: unblocked });
    }
  }).fail((xhr) => {
    if (latestSettingsSave && latestSettingsSave.state() === 'pending') return;

    const reason = (xhr.responseJSON && xhr.responseJSON.reason) || '';
    const bn = app.settings.get('blockedNodes');
    const failedBlocks = pendingBlocks.filter((peerID) => !bn.includes(peerID));
    const failedUnblocks = pendingUnblocks.filter((peerID) => bn.includes(peerID));
    pendingBlocks = [];
    pendingUnblocks = [];

    if (failedBlocks.length) {
      events.trigger('blockFail', {
        peerIDs: failedBlocks,
        reason,
      });
    }

    if (failedUnblocks.length) {
      events.trigger('unblockFail', {
        peerIDs: failedUnblocks,
        reason,
      });
    }

    if (failedBlocks.length || failedUnblocks.length) {
      let title;
      let body;

      if (failedBlocks.length && failedUnblocks.length) {
        title = app.polyglot.t('block.errorModal.titleUnableToBlockUnblock');
        body = `${app.polyglot.t('block.errorModal.blockFailedListHeading')}<br /><br />`
          + `<div class="txCtr">${failedBlocks.join('<br />')}</div><br />`
          + `${app.polyglot.t('block.errorModal.unblockFailedListHeading')}<br /><br />`
          + `<div class="txCtr">${failedUnblocks.join('<br />')}</div>`;
      } else if (failedUnblocks.length) {
        title = app.polyglot.t('block.errorModal.titleUnableToUnblock');
        body = `${app.polyglot.t('block.errorModal.unblockFailedListHeading')}<br /><br />`
          + `<div class="txCtr">${failedUnblocks.join('<br />')}</div>`;
      } else {
        title = app.polyglot.t('block.errorModal.titleUnableToBlock');
        body = `${app.polyglot.t('block.errorModal.blockFailedListHeading')}<br /><br />`
          + `<div class="txCtr">${failedBlocks.join('<br />')}</div>`;
      }

      if (reason) {
        body += `<br />${app.polyglot.t('block.errorModal.reason', { reason })}`;
      }

      openSimpleMessage(title, '', { messageHtml: body });
    }
  });

  events.trigger(_block ? 'blocking' : 'unblocking', { peerIDs: peerIDList });
}

export function block(peerIDs) {
  blockUnblock(true, peerIDs);
}

export function unblock(peerIDs) {
  blockUnblock(false, peerIDs);
}

export function isBlocked(peerID) {
  if (typeof peerID !== 'string') {
    throw new Error('Please provide a peerID as a string.');
  }

  checkAppSettings();

  return app.settings.get('blockedNodes').includes(peerID);
}

export function isBlocking(peerID) {
  if (typeof peerID !== 'string') {
    throw new Error('Please provide a peerID as a string.');
  }

  checkAppSettings();

  return (latestSettingsSave && latestSettingsSave.state() === 'pending' && lastSentBlockedNodes.includes(peerID)) || false;
}

export function isUnblocking(peerID) {
  if (typeof peerID !== 'string') {
    throw new Error('Please provide a peerID as a string.');
  }

  checkAppSettings();

  return (latestSettingsSave && latestSettingsSave.state() === 'pending'
    && !lastSentBlockedNodes.includes(peerID)
    && app.settings.get('blockedNodes').includes(peerID)) || false;
}
