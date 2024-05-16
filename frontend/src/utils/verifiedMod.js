import app from '../../backbone/app';
import VerifiedMod from '../../backbone/models/VerifiedMod';

function getBaseOptions(options = {}) {
    const opts = {
      shortText: true,
      shortTipTitle: false,
      verified: !!options.model,
      ...options,
    };
  
    if (opts.model
      && !(opts.model instanceof VerifiedMod)) {
      throw new Error('If providing a model, it should be an instance of '
        + 'a VerifiedMod model.');
    }
  
    const textKey = opts.shortText
      ? 'titleShort' : 'titleLong';
  
    const tipTitleKey = opts.shortTipTitle
      ? 'titleShort' : 'titleLong';
  
    return {
      badge: (opts.model && opts.model.get('type').badge) || undefined,
      initialState: {
        verified: opts.verified,
        text: opts.verified
          ? app.polyglot.t(`verifiedMod.modVerified.${textKey}`)
          : app.polyglot.t(`verifiedMod.modUnverified.${textKey}`),
        tipTitle: opts.verified
          ? app.polyglot.t(`verifiedMod.modVerified.${tipTitleKey}`)
          : app.polyglot.t(`verifiedMod.modUnverified.${tipTitleKey}`),
      },
    };
  }
  
  export function getModeratorOptions(options = {}) {
    const opts = {
      verified: !!options.model,
      ...options,
    };
  
    const baseOptions = getBaseOptions(options);
  
    return {
      ...baseOptions,
      initialState: {
        ...baseOptions.initialState,
        tipBody: opts.verified
          ? app.polyglot.t('verifiedMod.modVerified.tipBody', {
            name: `<b>${app.verifiedMods.data.name}</b>`,
            link: app.verifiedMods.data.link
              ? `<a class="txU noWrap" href="${app.verifiedMods.data.link}" target="_blank">`
                + `${app.polyglot.t('verifiedMod.modVerified.link')}</a>`
              : '',
          })
          : app.polyglot.t('verifiedMod.modUnverified.tipBody', {
            name: `<b>${app.verifiedMods.data.name}</b>`,
            not: `<b>${app.polyglot.t('verifiedMod.modUnverified.not')}</b>`,
          }),
      },
    };
  }
  
  export function getListingOptions(options = {}) {
    const opts = {
      verified: !!options.model,
      ...options,
    };
  
    const baseOptions = getBaseOptions(options);
  
    return {
      ...baseOptions,
      initialState: {
        ...baseOptions.initialState,
        text: '',
        tipBody: opts.verified
          ? app.polyglot.t('verifiedMod.listingVerified.tipBody', {
            name: `<b>${app.verifiedMods.data.name}</b>`,
            link: app.verifiedMods.data.link
              ? `<a class="txU noWrap" href="${app.verifiedMods.data.link}" data-open-external>`
                + `${app.polyglot.t('verifiedMod.listingVerified.link')}</a>`
              : '',
          })
          : app.polyglot.t('verifiedMod.listingUnverified.tipBody', {
            name: `<b>${app.verifiedMods.data.name}</b>`,
            not: `<b>${app.polyglot.t('verifiedMod.listingUnverified.not')}</b>`,
          }),
      },
    };
  }