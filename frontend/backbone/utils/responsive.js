import app from '../app';

// functions for determining size and resolution of the window

export function isHiRez() {
  return window.matchMedia('(-webkit-min-device-pixel-ratio: 1.5)').matches;
}

export function isLargeWidth() {
  return window.matchMedia('(min-width: 1500px)').matches;
}

export function isSmallHeight() {
  return window.matchMedia('(max-height: 700px)').matches;
}

function getBackgroundImage(imageHashes = {}, standardSize, responsiveSize, defaultUrl, needUrl = false) {
  let imageHash = '';
  let bgImageProperty = '';

  if (isHiRez() && imageHashes && imageHashes[responsiveSize]) {
    imageHash = imageHashes[responsiveSize];
  } else if (imageHashes && imageHashes[standardSize]) {
    imageHash = imageHashes[standardSize];
  }

  if (imageHash) {
    bgImageProperty = needUrl ? app.getServerUrl(`ob/image/${imageHash}`) : `background-image: url(${app.getServerUrl(`ob/image/${imageHash}`)})` +
      `, url(${defaultUrl})`;
  } else {
    bgImageProperty = needUrl ? defaultUrl : `background-image: url(${defaultUrl})`;
  }

  return bgImageProperty;
}

export function getAvatarBgImage(avatarHashes = {}, options = {}, needUrl = false) {
  const opts = {
    standardSize: 'tiny',
    responsiveSize: 'small',
    defaultUrl: '../imgs/defaultAvatar.png',
    ...options,
  };

  return getBackgroundImage(avatarHashes, opts.standardSize, opts.responsiveSize,
    opts.defaultUrl, needUrl);
}

export function getListingBgImage(imageHashes = {}, options = {}, needUrl = false) {
  const opts = {
    standardSize: 'tiny',
    responsiveSize: 'small',
    defaultUrl: '../imgs/defaultItem.png',
    ...options,
  };

  return getBackgroundImage(imageHashes, opts.standardSize, opts.responsiveSize,
    opts.defaultUrl, needUrl);
}

