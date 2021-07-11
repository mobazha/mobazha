import {I18n} from '../langs/I18n';

export const DEFAULT_FEE_LEVELS = [
  {
    label: I18n.t('utils.fee.Super_Economic'),
    value: 'superEconomic',
    description: I18n.t('utils.fee.Super_economic_v'),
    fee: 0,
  },
  {
    label: I18n.t('utils.fee.Economic'),
    value: 'economic',
    description: I18n.t('utils.fee.Economic_v'),
    fee: 0,
  },
  {
    label: I18n.t('utils.fee.Normal'),
    value: 'normal',
    description: I18n.t('utils.fee.Normal_v'),
    fee: 0,
  },
  {
    label: I18n.t('utils.fee.Priority'),
    value: 'priority',
    description: I18n.t('utils.fee.Priority_v'),
    fee: 0,
  },
];

export const getFeeLevelDescription = (level) => {
  switch (level) {
    case 'superEconomic':
      return I18n.t('utils.fee.Super_economic_v');
    case 'priority':
      return I18n.t('utils.fee.Priority_v');
    case 'economic':
      return I18n.t('utils.fee.Economic_v');
    case 'normal':
    default:
      return I18n.t('utils.fee.Normal_v');
  }
};
